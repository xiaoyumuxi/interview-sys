package interview

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ai-interview-platform/internal/contextengine"
	airuntime "ai-interview-platform/internal/runtime"
	"ai-interview-platform/internal/singleflight"
	"ai-interview-platform/internal/store"
	"ai-interview-platform/internal/workqueue"
)

const (
	SessionReady      = "READY"
	SessionInProgress = "IN_PROGRESS"
	SessionFinished   = "FINISHED"
	SessionFailed     = "FAILED"

	FlowInit       = "INIT"
	FlowAsking     = "ASKING"
	FlowEvaluating = "EVALUATING"
	FlowFollowUp   = "FOLLOW_UP"
	FlowCompleted  = "COMPLETED"

	TurnQueued    = "queued"
	TurnRunning   = "running"
	TurnCompleted = "completed"
	TurnFailed    = "failed"
)

type Service struct {
	db      *sql.DB
	store   *store.Store
	engine  *contextengine.Engine
	runtime *airuntime.Client
	flights *singleflight.RedisFlight
	queue   *workqueue.Stream
}

func NewService(db *sql.DB, store *store.Store, engine *contextengine.Engine, runtime *airuntime.Client, flights *singleflight.RedisFlight, queue *workqueue.Stream) *Service {
	return &Service{db: db, store: store, engine: engine, runtime: runtime, flights: flights, queue: queue}
}

func (s *Service) StartWorker(ctx context.Context) {
	const group = "interview-workers"
	consumer := "api-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	s.queue.EnsureGroup(ctx, group)
	go s.dispatchOutboxLoop(ctx)
	go s.reclaimStaleTurnsLoop(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			messages, err := s.queue.ReadGroup(ctx, group, consumer, 8, 2*time.Second)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			for _, message := range messages {
				if s.handleStreamMessage(ctx, message.Values) {
					s.queue.Ack(ctx, group, message.ID)
				}
			}
		}
	}()
}

type outboxMessage struct {
	MessageID   string
	EventType   string
	AggregateID string
	Payload     []byte
	Attempts    int
	MaxAttempts int
}

func (s *Service) dispatchOutboxLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		token, ok, err := s.queue.TryLock(ctx, "lock:async_messages:dispatch:"+s.queue.Name(), 10*time.Second)
		if err != nil || !ok {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		s.dispatchOutboxBatch(ctx, 25)
		s.queue.Unlock(context.Background(), "lock:async_messages:dispatch:"+s.queue.Name(), token)
		time.Sleep(300 * time.Millisecond)
	}
}

func (s *Service) dispatchOutboxBatch(ctx context.Context, limit int) {
	messages, err := s.claimOutboxMessages(ctx, limit)
	if err != nil {
		return
	}
	for _, message := range messages {
		var payload map[string]any
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			s.markOutboxFailed(ctx, message, err)
			continue
		}
		redisID, err := s.queue.PublishWithID(ctx, workqueue.Event{
			Type:      message.EventType,
			SessionID: message.AggregateID,
			Payload:   payload,
		})
		if err != nil {
			s.releaseOutboxForRetry(ctx, message, err)
			continue
		}
		_, _ = s.db.ExecContext(ctx, `
UPDATE async_messages
SET status='dispatched', redis_message_id=$2, dispatched_at=now(),
    last_error='', updated_at=now()
WHERE message_id=$1`, message.MessageID, redisID)
	}
}

func (s *Service) claimOutboxMessages(ctx context.Context, limit int) ([]outboxMessage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `
SELECT message_id, event_type, aggregate_id, payload, attempts, max_attempts
FROM async_messages
WHERE status IN ('pending','failed')
  AND next_retry_at <= now()
  AND attempts < max_attempts
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}
	var messages []outboxMessage
	for rows.Next() {
		var message outboxMessage
		if err := rows.Scan(&message.MessageID, &message.EventType, &message.AggregateID, &message.Payload, &message.Attempts, &message.MaxAttempts); err != nil {
			_ = rows.Close()
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, message := range messages {
		if _, err := tx.ExecContext(ctx, `
UPDATE async_messages
SET status='dispatching', attempts=attempts+1, updated_at=now()
WHERE message_id=$1`, message.MessageID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *Service) releaseOutboxForRetry(ctx context.Context, message outboxMessage, cause error) {
	delaySeconds := (message.Attempts + 1) * 5
	if delaySeconds > 60 {
		delaySeconds = 60
	}
	_, _ = s.db.ExecContext(ctx, `
UPDATE async_messages
SET status='pending', next_retry_at=now() + ($2 * interval '1 second'),
    last_error=$3, updated_at=now()
WHERE message_id=$1`, message.MessageID, delaySeconds, cause.Error())
}

func (s *Service) markOutboxFailed(ctx context.Context, message outboxMessage, cause error) {
	_, _ = s.db.ExecContext(ctx, `
UPDATE async_messages
SET status='failed', last_error=$2, updated_at=now()
WHERE message_id=$1`, message.MessageID, cause.Error())
}

func (s *Service) reclaimStaleTurnsLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
			s.reclaimStaleTurns(ctx)
		}
	}
}

func (s *Service) reclaimStaleTurns(ctx context.Context) {
	if err := validateTurnTransition(TurnRunning, TurnQueued); err != nil {
		return
	}
	rows, err := s.db.QueryContext(ctx, `
UPDATE interview_turns
SET turn_status=$2, error_text='requeued after stale running state', updated_at=now()
WHERE turn_status=$1 AND updated_at < now() - interval '2 minutes'
RETURNING turn_id`, TurnRunning, TurnQueued)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var turnID string
		if err := rows.Scan(&turnID); err != nil {
			continue
		}
		_, _ = s.db.ExecContext(ctx, `
UPDATE async_messages
SET status='pending', next_retry_at=now(), last_error='requeued after stale running turn', updated_at=now()
WHERE dedup_key=$1`, "interview.answer_submitted:"+turnID)
	}
}

type CreateSessionRequest struct {
	UserID       string         `json:"user_id"`
	SkillID      string         `json:"skill_id"`
	QuestionType string         `json:"question_type"`
	MaxFollowUps int            `json:"max_follow_ups"`
	Metadata     map[string]any `json:"metadata"`
}

type SubmitAnswerRequest struct {
	RequestID      string `json:"request_id"`
	QuestionID     string `json:"question_id"`
	QuestionNumber int    `json:"question_number"`
	AnswerRound    int    `json:"answer_round"`
	UserAnswer     string `json:"user_answer"`
	DryRun         bool   `json:"dry_run"`
}

type Session struct {
	SessionID             string         `json:"session_id"`
	UserID                string         `json:"user_id"`
	SkillID               string         `json:"skill_id"`
	SessionStatus         string         `json:"session_status"`
	FlowStatus            string         `json:"flow_status"`
	Phase                 string         `json:"phase"`
	CurrentQuestionID     string         `json:"current_question_id,omitempty"`
	CurrentQuestionNumber int            `json:"current_question_number"`
	AnswerRound           int            `json:"answer_round"`
	FollowUpCount         int            `json:"follow_up_count"`
	MaxFollowUps          int            `json:"max_follow_ups"`
	TotalScore            float64        `json:"total_score"`
	Metadata              map[string]any `json:"metadata"`
	LastError             string         `json:"last_error,omitempty"`
	CreatedAt             string         `json:"created_at"`
	UpdatedAt             string         `json:"updated_at"`
	FinishedAt            string         `json:"finished_at,omitempty"`
	CurrentQuestion       *Question      `json:"current_question,omitempty"`
	Turns                 []Turn         `json:"turns,omitempty"`
}

type Question struct {
	QuestionID string   `json:"question_id"`
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	Prompt     string   `json:"prompt"`
	Tags       []string `json:"tags"`
}

type Turn struct {
	TurnID           string         `json:"turn_id"`
	SessionID        string         `json:"session_id"`
	QuestionID       string         `json:"question_id,omitempty"`
	QuestionNumber   int            `json:"question_number"`
	AnswerRound      int            `json:"answer_round"`
	RequestID        string         `json:"request_id"`
	AnswerHash       string         `json:"answer_hash"`
	UserAnswer       string         `json:"user_answer"`
	Evaluation       map[string]any `json:"evaluation"`
	FollowUpNeeded   bool           `json:"follow_up_needed"`
	FollowUpQuestion string         `json:"follow_up_question,omitempty"`
	Score            float64        `json:"score"`
	TraceID          string         `json:"trace_id,omitempty"`
	Response         map[string]any `json:"response"`
	TurnStatus       string         `json:"turn_status"`
	ErrorText        string         `json:"error_text,omitempty"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error) {
	if strings.TrimSpace(req.SkillID) == "" {
		return Session{}, errors.New("skill_id is required")
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = "local-user"
	}
	if req.MaxFollowUps <= 0 {
		req.MaxFollowUps = 1
	}
	questionType := strings.TrimSpace(req.QuestionType)
	if questionType == "" {
		questionType = "algorithm"
	}
	question, ok, err := s.questionByNumber(ctx, questionType, 1)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, fmt.Errorf("no published question found for question_type=%s", questionType)
	}
	sessionID := store.NewID("sess")
	meta := cloneMap(req.Metadata)
	meta["question_type"] = questionType
	metaJSON, _ := json.Marshal(meta)
	_, err = s.db.ExecContext(ctx, `
INSERT INTO interview_sessions (
  session_id, user_id, skill_id, session_status, flow_status, phase,
  current_question_id, current_question_number, answer_round, max_follow_ups, metadata, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())`,
		sessionID, userID, req.SkillID, SessionReady, FlowAsking, "technical",
		question.QuestionID, 1, 0, req.MaxFollowUps, metaJSON,
	)
	if err != nil {
		return Session{}, err
	}
	_ = s.enqueueOutbox(ctx, workqueue.Event{Type: "interview.session_created", SessionID: sessionID, Payload: map[string]any{"skill_id": req.SkillID, "question_id": question.QuestionID}}, "interview.session_created:"+sessionID, "interview_session", sessionID)
	if err := s.refreshSnapshot(ctx, sessionID, "create_session"); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, sessionID)
}

func (s *Service) GetSession(ctx context.Context, sessionID string) (Session, error) {
	session, err := s.loadSession(ctx, sessionID, true)
	if err != nil {
		return Session{}, err
	}
	if session.SessionID == "" {
		return Session{}, sql.ErrNoRows
	}
	return session, nil
}

func (s *Service) SubmitAnswer(ctx context.Context, sessionID string, req SubmitAnswerRequest) (map[string]any, error) {
	answer := strings.TrimSpace(req.UserAnswer)
	if answer == "" {
		return nil, errors.New("user_answer is required")
	}
	session, err := s.loadSession(ctx, sessionID, false)
	if err != nil {
		return nil, err
	}
	if session.SessionID == "" {
		return nil, sql.ErrNoRows
	}
	answerHash := hashAnswer(answer)
	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		requestID = "auto_" + answerHash[:16]
	}
	if replay, ok, err := s.findReplay(ctx, sessionID, requestID, session.CurrentQuestionNumber, session.AnswerRound, answerHash); err != nil {
		return nil, err
	} else if ok {
		return replay, nil
	}
	if req.QuestionID != "" && req.QuestionID != session.CurrentQuestionID {
		return nil, errors.New("stale question_id, please refresh current session")
	}
	if req.QuestionNumber > 0 && req.QuestionNumber != session.CurrentQuestionNumber {
		return nil, errors.New("stale question_number, please refresh current session")
	}
	if req.AnswerRound > 0 && req.AnswerRound != session.AnswerRound {
		return nil, errors.New("stale answer_round, please refresh current session")
	}
	if session.SessionStatus == SessionFinished || session.FlowStatus == FlowCompleted {
		return nil, errors.New("interview session is already finished")
	}
	if err := validateSessionTransition(session.SessionStatus, SessionInProgress); err != nil {
		return nil, err
	}
	if err := validateFlowTransition(session.FlowStatus, FlowEvaluating); err != nil {
		return nil, err
	}

	response, _, err := s.enqueueAnswer(ctx, session, req, requestID, answerHash, answer)
	if err != nil {
		if replay, ok, replayErr := s.findReplay(ctx, sessionID, requestID, session.CurrentQuestionNumber, session.AnswerRound, answerHash); replayErr != nil {
			return nil, replayErr
		} else if ok {
			return replay, nil
		}
		return nil, err
	}
	return response, nil
}

func (s *Service) Finalize(ctx context.Context, sessionID string) (Session, error) {
	current, err := s.loadSession(ctx, sessionID, false)
	if err != nil {
		return Session{}, err
	}
	if current.SessionID == "" {
		return Session{}, sql.ErrNoRows
	}
	if err := validateSessionTransition(current.SessionStatus, SessionFinished); err != nil {
		return Session{}, err
	}
	if err := validateFlowTransition(current.FlowStatus, FlowCompleted); err != nil {
		return Session{}, err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE interview_sessions
SET session_status=$2, flow_status=$3, finished_at=COALESCE(finished_at, now()), updated_at=now()
WHERE session_id=$1`,
		sessionID, SessionFinished, FlowCompleted)
	if err != nil {
		return Session{}, err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return Session{}, sql.ErrNoRows
	}
	_ = s.enqueueOutbox(ctx, workqueue.Event{Type: "interview.session_finalized", SessionID: sessionID, Payload: map[string]any{"source": "api"}}, "interview.session_finalized:"+sessionID, "interview_session", sessionID)
	_ = s.refreshSnapshot(ctx, sessionID, "finalize")
	return s.GetSession(ctx, sessionID)
}

func (s *Service) Trace(ctx context.Context, sessionID string) ([]Turn, error) {
	return s.loadTurns(ctx, sessionID)
}

func (s *Service) enqueueAnswer(ctx context.Context, session Session, req SubmitAnswerRequest, requestID string, answerHash string, answer string) (map[string]any, string, error) {
	turnID := store.NewID("turn")
	response := map[string]any{
		"schema_version":  "interview.answer.accepted.v1",
		"session_id":      session.SessionID,
		"turn_id":         turnID,
		"turn_status":     TurnQueued,
		"request_id":      requestID,
		"question_id":     session.CurrentQuestionID,
		"question_number": session.CurrentQuestionNumber,
		"answer_round":    session.AnswerRound,
		"accepted":        true,
		"dry_run":         req.DryRun,
		"poll_url":        "/api/interview-sessions/" + session.SessionID + "/trace",
		"stream_event":    "interview.answer_submitted",
	}
	responseJSON, _ := json.Marshal(response)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
INSERT INTO interview_turns (
  turn_id, session_id, question_id, question_number, answer_round, request_id, answer_hash,
  user_answer, turn_status, response, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now())`,
		turnID, session.SessionID, nullEmpty(session.CurrentQuestionID), session.CurrentQuestionNumber,
		session.AnswerRound, requestID, answerHash, answer, TurnQueued, responseJSON,
	)
	if err != nil {
		return nil, "", err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE interview_sessions
SET session_status=$2, flow_status=$3, updated_at=now()
WHERE session_id=$1 AND flow_status <> $4`,
		session.SessionID, SessionInProgress, FlowEvaluating, FlowCompleted)
	if err != nil {
		return nil, "", err
	}
	if err := s.enqueueOutboxTx(ctx, tx, workqueue.Event{
		Type:      "interview.answer_submitted",
		SessionID: session.SessionID,
		Payload: map[string]any{
			"turn_id":         turnID,
			"request_id":      requestID,
			"question_number": session.CurrentQuestionNumber,
			"answer_round":    session.AnswerRound,
		},
	}, "interview.answer_submitted:"+turnID, "interview_turn", turnID); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	_ = s.refreshSnapshot(ctx, session.SessionID, "answer_queued")
	return response, turnID, nil
}

func (s *Service) enqueueOutbox(ctx context.Context, event workqueue.Event, dedupKey string, aggregateType string, aggregateID string) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO async_messages (
  message_id, stream_name, event_type, aggregate_type, aggregate_id, dedup_key, payload, status, next_retry_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',now(),now())
ON CONFLICT (stream_name, dedup_key) DO NOTHING`,
		store.NewID("msg"), s.queue.Name(), event.Type, aggregateType, aggregateID, dedupKey, payload)
	return err
}

func (s *Service) enqueueOutboxTx(ctx context.Context, tx *sql.Tx, event workqueue.Event, dedupKey string, aggregateType string, aggregateID string) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO async_messages (
  message_id, stream_name, event_type, aggregate_type, aggregate_id, dedup_key, payload, status, next_retry_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',now(),now())
ON CONFLICT (stream_name, dedup_key) DO NOTHING`,
		store.NewID("msg"), s.queue.Name(), event.Type, aggregateType, aggregateID, dedupKey, payload)
	return err
}

func (s *Service) handleStreamMessage(ctx context.Context, values map[string]any) bool {
	eventType, _ := values["event_type"].(string)
	if eventType != "interview.answer_submitted" {
		return true
	}
	payloadText, _ := values["payload"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
		return true
	}
	turnID := stringFromMap(payload, "turn_id")
	if turnID == "" {
		return true
	}
	return s.ProcessTurn(ctx, turnID) == nil
}

func (s *Service) ProcessTurn(ctx context.Context, turnID string) error {
	turn, err := s.loadTurn(ctx, turnID)
	if err != nil {
		return err
	}
	if turn.TurnID == "" || turn.TurnStatus == TurnCompleted {
		return nil
	}
	if turn.TurnStatus == TurnFailed {
		return nil
	}
	session, err := s.loadSession(ctx, turn.SessionID, false)
	if err != nil {
		return err
	}
	if session.SessionID == "" {
		return sql.ErrNoRows
	}
	lockKey := "lock:turn:" + turnID
	lockToken, ok, err := s.queue.TryLock(ctx, lockKey, 2*time.Minute)
	if err != nil || !ok {
		return err
	}
	defer s.queue.Unlock(context.Background(), lockKey, lockToken)
	claimed, err := s.claimTurn(ctx, turnID)
	if err != nil || !claimed {
		return err
	}
	turn.TurnStatus = TurnRunning
	flightKey := strings.Join([]string{
		"interview-evaluation",
		turn.SessionID,
		strconv.Itoa(turn.QuestionNumber),
		strconv.Itoa(turn.AnswerRound),
		turn.AnswerHash,
	}, "|")
	result, err := s.flights.Execute(ctx, flightKey, func(runCtx context.Context) (string, error) {
		payload, err := s.evaluateAnswer(runCtx, session, turn.UserAnswer, boolFromMap(turn.Response, "dry_run"))
		if err != nil {
			return "", err
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	})
	if err != nil {
		if errors.Is(err, singleflight.ErrInFlight) {
			if transitionErr := validateTurnTransition(TurnRunning, TurnQueued); transitionErr != nil {
				return transitionErr
			}
			_, _ = s.db.ExecContext(ctx, `UPDATE interview_turns SET turn_status=$2, updated_at=now() WHERE turn_id=$1`, turnID, TurnQueued)
			return err
		}
		_ = s.markTurnFailed(ctx, turnID, turn.SessionID, err)
		return nil
	}
	var evaluationPayload evaluationPayload
	if err := json.Unmarshal([]byte(result.Value), &evaluationPayload); err != nil {
		_ = s.markTurnFailed(ctx, turnID, turn.SessionID, err)
		return nil
	}
	return s.persistAnswerResult(ctx, session, turn, evaluationPayload, result.Replay)
}

func (s *Service) claimTurn(ctx context.Context, turnID string) (bool, error) {
	if err := validateTurnTransition(TurnQueued, TurnRunning); err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE interview_turns
SET turn_status=$2, processing_attempts=processing_attempts+1, updated_at=now()
WHERE turn_id=$1 AND turn_status=$3`,
		turnID, TurnRunning, TurnQueued)
	if err != nil {
		return false, err
	}
	count, _ := result.RowsAffected()
	return count > 0, nil
}

func (s *Service) markTurnFailed(ctx context.Context, turnID string, sessionID string, cause error) error {
	turn, err := s.loadTurn(ctx, turnID)
	if err != nil {
		return err
	}
	if turn.TurnID == "" {
		return sql.ErrNoRows
	}
	session, err := s.loadSession(ctx, sessionID, false)
	if err != nil {
		return err
	}
	if session.SessionID == "" {
		return sql.ErrNoRows
	}
	if err := validateTurnTransition(turn.TurnStatus, TurnFailed); err != nil {
		return err
	}
	if err := validateSessionTransition(session.SessionStatus, SessionFailed); err != nil {
		return err
	}
	message := cause.Error()
	response := map[string]any{
		"schema_version": "interview.answer.failed.v1",
		"session_id":     sessionID,
		"turn_id":        turnID,
		"turn_status":    TurnFailed,
		"error":          message,
	}
	raw, _ := json.Marshal(response)
	result, err := s.db.ExecContext(ctx, `
UPDATE interview_turns
SET turn_status=$2, error_text=$3, response=$4, updated_at=now()
WHERE turn_id=$1 AND turn_status=$5`, turnID, TurnFailed, message, raw, turn.TurnStatus)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errors.New("turn state changed before failure could be recorded")
	}
	_, _ = s.db.ExecContext(ctx, `
UPDATE interview_sessions
SET session_status=$2, last_error=$3, updated_at=now()
WHERE session_id=$1`, sessionID, SessionFailed, message)
	_ = s.enqueueOutbox(ctx, workqueue.Event{Type: "interview.answer_failed", SessionID: sessionID, Payload: response}, "interview.answer_failed:"+turnID, "interview_turn", turnID)
	_ = s.refreshSnapshot(ctx, sessionID, "answer_failed")
	return nil
}

type evaluationPayload struct {
	RuntimeResponse  airuntime.TaskResponse        `json:"runtime_response"`
	ContextPreview   contextengine.PreviewResponse `json:"context_preview"`
	TraceID          string                        `json:"trace_id"`
	Evaluation       map[string]any                `json:"evaluation"`
	Score            float64                       `json:"score"`
	FollowUpNeeded   bool                          `json:"follow_up_needed"`
	FollowUpQuestion string                        `json:"follow_up_question"`
}

func (s *Service) evaluateAnswer(ctx context.Context, session Session, answer string, dryRun bool) (evaluationPayload, error) {
	preview, err := s.engine.Preview(ctx, contextengine.PreviewRequest{
		TaskType:  "answer_evaluation",
		SkillID:   session.SkillID,
		SessionID: session.SessionID,
	})
	if err != nil {
		return evaluationPayload{}, err
	}
	provider, err := s.store.RuntimeProviderForTask(ctx, "answer_evaluation")
	if err != nil {
		return evaluationPayload{}, err
	}
	userInput := fmt.Sprintf("Question: %s\n\nCandidate answer: %s", questionText(session.CurrentQuestion), answer)
	runtimeResp, err := s.runtime.RunTask(ctx, airuntime.TaskRequest{
		TaskType:     "answer_evaluation",
		Provider:     provider,
		ContextItems: preview.Items,
		UserInput:    userInput,
		DryRun:       dryRun,
	})
	if err != nil {
		return evaluationPayload{}, err
	}
	eval := runtimeResp.Output
	score := floatFromMap(eval, "score")
	followUpNeeded := boolFromMap(eval, "follow_up_needed")
	followUpQuestion := stringFromMap(eval, "follow_up_question")
	if followUpQuestion == "" {
		followUpQuestion = stringFromMap(eval, "next_question")
	}
	traceID := store.NewID("trace")
	_ = s.store.InsertAgentTrace(ctx, store.TraceRecord{
		TraceID:      traceID,
		TaskType:     "answer_evaluation",
		SkillID:      session.SkillID,
		Input:        map[string]any{"session_id": session.SessionID, "question_id": session.CurrentQuestionID},
		ContextItems: preview.Items,
		Output:       runtimeResp,
	})
	return evaluationPayload{
		RuntimeResponse:  runtimeResp,
		ContextPreview:   preview,
		TraceID:          traceID,
		Evaluation:       eval,
		Score:            score,
		FollowUpNeeded:   followUpNeeded,
		FollowUpQuestion: followUpQuestion,
	}, nil
}

func (s *Service) persistAnswerResult(ctx context.Context, session Session, turn Turn, payload evaluationPayload, replay bool) error {
	nextFlow := FlowAsking
	nextStatus := SessionInProgress
	nextQuestionID := session.CurrentQuestionID
	nextQuestionNumber := session.CurrentQuestionNumber
	nextAnswerRound := session.AnswerRound + 1
	nextFollowUpCount := session.FollowUpCount
	finished := false
	followUpNeeded := payload.FollowUpNeeded && payload.FollowUpQuestion != "" && session.FollowUpCount < session.MaxFollowUps
	if followUpNeeded {
		nextFlow = FlowFollowUp
		nextFollowUpCount++
	} else {
		questionType := stringFromMap(session.Metadata, "question_type")
		if questionType == "" {
			questionType = "algorithm"
		}
		nextQuestion, ok, err := s.questionByNumber(ctx, questionType, session.CurrentQuestionNumber+1)
		if err != nil {
			return err
		}
		if ok {
			nextQuestionID = nextQuestion.QuestionID
			nextQuestionNumber = nextQuestion.Number
			nextAnswerRound = 0
			nextFollowUpCount = 0
			payload.FollowUpQuestion = ""
		} else {
			nextFlow = FlowCompleted
			nextStatus = SessionFinished
			nextQuestionID = ""
			finished = true
		}
	}
	if err := validateTurnTransition(turn.TurnStatus, TurnCompleted); err != nil {
		return err
	}
	if err := validateSessionTransition(session.SessionStatus, nextStatus); err != nil {
		return err
	}
	if err := validateFlowTransition(session.FlowStatus, nextFlow); err != nil {
		return err
	}
	response := map[string]any{
		"schema_version":       "interview.answer.v1",
		"session_id":           session.SessionID,
		"turn_id":              turn.TurnID,
		"turn_status":          TurnCompleted,
		"trace_id":             payload.TraceID,
		"score":                payload.Score,
		"evaluation":           payload.Evaluation,
		"follow_up_needed":     followUpNeeded,
		"follow_up_question":   payload.FollowUpQuestion,
		"finished":             finished,
		"singleflight_replay":  replay,
		"next_question_id":     nextQuestionID,
		"next_question_number": nextQuestionNumber,
		"next_answer_round":    nextAnswerRound,
	}
	responseJSON, _ := json.Marshal(response)
	evalJSON, _ := json.Marshal(payload.Evaluation)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
UPDATE interview_turns
SET turn_status=$2, evaluation=$3, follow_up_needed=$4, follow_up_question=$5,
    score=$6, trace_id=$7, response=$8, error_text='', updated_at=now()
WHERE turn_id=$1 AND turn_status=$9`,
		turn.TurnID, TurnCompleted, evalJSON, followUpNeeded,
		payload.FollowUpQuestion, payload.Score, nullEmpty(payload.TraceID), responseJSON, TurnRunning,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE interview_sessions
SET session_status=$2, flow_status=$3, current_question_id=$4, current_question_number=$5,
    answer_round=$6, follow_up_count=$7, total_score=total_score+$8,
    finished_at=CASE WHEN $9 THEN COALESCE(finished_at, now()) ELSE finished_at END,
    updated_at=now()
WHERE session_id=$1`,
		session.SessionID, nextStatus, nextFlow, nullEmpty(nextQuestionID), nextQuestionNumber,
		nextAnswerRound, nextFollowUpCount, payload.Score, finished,
	)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_ = s.enqueueOutbox(ctx, workqueue.Event{Type: "interview.answer_evaluated", SessionID: session.SessionID, Payload: response}, "interview.answer_evaluated:"+turn.TurnID, "interview_turn", turn.TurnID)
	_ = s.refreshSnapshot(ctx, session.SessionID, "answer")
	return nil
}

func (s *Service) loadSession(ctx context.Context, sessionID string, includeTurns bool) (Session, error) {
	var item Session
	var metadataRaw []byte
	var currentQuestionID sql.NullString
	var createdAt, updatedAt time.Time
	var finishedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
SELECT session_id, COALESCE(user_id,''), skill_id, session_status, flow_status, phase,
       current_question_id, current_question_number, answer_round, follow_up_count, max_follow_ups,
       total_score, metadata, last_error, created_at, updated_at, finished_at
FROM interview_sessions
WHERE session_id=$1`, sessionID).Scan(
		&item.SessionID, &item.UserID, &item.SkillID, &item.SessionStatus, &item.FlowStatus, &item.Phase,
		&currentQuestionID, &item.CurrentQuestionNumber, &item.AnswerRound, &item.FollowUpCount, &item.MaxFollowUps,
		&item.TotalScore, &metadataRaw, &item.LastError, &createdAt, &updatedAt, &finishedAt,
	)
	if err == sql.ErrNoRows {
		return Session{}, nil
	}
	if err != nil {
		return Session{}, err
	}
	item.CurrentQuestionID = currentQuestionID.String
	_ = json.Unmarshal(metadataRaw, &item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)
	if finishedAt.Valid {
		item.FinishedAt = finishedAt.Time.Format(time.RFC3339)
	}
	if item.CurrentQuestionID != "" {
		question, ok, err := s.questionByID(ctx, item.CurrentQuestionID, item.CurrentQuestionNumber)
		if err != nil {
			return Session{}, err
		}
		if ok {
			item.CurrentQuestion = &question
		}
	}
	if includeTurns {
		turns, err := s.loadTurns(ctx, sessionID)
		if err != nil {
			return Session{}, err
		}
		item.Turns = turns
	}
	return item, nil
}

func (s *Service) loadTurns(ctx context.Context, sessionID string) ([]Turn, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT turn_id, session_id, COALESCE(question_id,''), question_number, answer_round, request_id, answer_hash,
       user_answer, turn_status, evaluation, follow_up_needed, follow_up_question, score, COALESCE(trace_id,''),
       response, error_text, created_at, updated_at
FROM interview_turns
WHERE session_id=$1
ORDER BY created_at`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []Turn
	for rows.Next() {
		var turn Turn
		var evalRaw, responseRaw []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&turn.TurnID, &turn.SessionID, &turn.QuestionID, &turn.QuestionNumber, &turn.AnswerRound,
			&turn.RequestID, &turn.AnswerHash, &turn.UserAnswer, &turn.TurnStatus, &evalRaw, &turn.FollowUpNeeded,
			&turn.FollowUpQuestion, &turn.Score, &turn.TraceID, &responseRaw, &turn.ErrorText, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(evalRaw, &turn.Evaluation)
		_ = json.Unmarshal(responseRaw, &turn.Response)
		turn.CreatedAt = createdAt.Format(time.RFC3339)
		turn.UpdatedAt = updatedAt.Format(time.RFC3339)
		turns = append(turns, turn)
	}
	if turns == nil {
		turns = []Turn{}
	}
	return turns, rows.Err()
}

func (s *Service) loadTurn(ctx context.Context, turnID string) (Turn, error) {
	var turn Turn
	var evalRaw, responseRaw []byte
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
SELECT turn_id, session_id, COALESCE(question_id,''), question_number, answer_round, request_id, answer_hash,
       user_answer, turn_status, evaluation, follow_up_needed, follow_up_question, score, COALESCE(trace_id,''),
       response, error_text, created_at, updated_at
FROM interview_turns
WHERE turn_id=$1`, turnID).Scan(
		&turn.TurnID, &turn.SessionID, &turn.QuestionID, &turn.QuestionNumber, &turn.AnswerRound,
		&turn.RequestID, &turn.AnswerHash, &turn.UserAnswer, &turn.TurnStatus, &evalRaw, &turn.FollowUpNeeded,
		&turn.FollowUpQuestion, &turn.Score, &turn.TraceID, &responseRaw, &turn.ErrorText, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return Turn{}, nil
	}
	if err != nil {
		return Turn{}, err
	}
	_ = json.Unmarshal(evalRaw, &turn.Evaluation)
	_ = json.Unmarshal(responseRaw, &turn.Response)
	turn.CreatedAt = createdAt.Format(time.RFC3339)
	turn.UpdatedAt = updatedAt.Format(time.RFC3339)
	return turn, nil
}

func (s *Service) findReplay(ctx context.Context, sessionID string, requestID string, questionNumber int, answerRound int, answerHash string) (map[string]any, bool, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT response
FROM interview_turns
WHERE session_id=$1 AND (request_id=$2 OR (question_number=$3 AND answer_round=$4 AND answer_hash=$5))
ORDER BY created_at
LIMIT 1`, sessionID, requestID, questionNumber, answerRound, answerHash).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var response map[string]any
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, false, err
	}
	response["idempotency_replay"] = true
	return response, true, nil
}

func (s *Service) questionByNumber(ctx context.Context, questionType string, number int) (Question, bool, error) {
	if number <= 0 {
		return Question{}, false, nil
	}
	var q Question
	var tags string
	err := s.db.QueryRowContext(ctx, `
SELECT question_id, title, prompt, array_to_string(topic_tags, ',')
FROM code_questions
WHERE status='published' AND question_type=$1
ORDER BY frequency_rank NULLS LAST, title
OFFSET $2 LIMIT 1`, questionType, number-1).Scan(&q.QuestionID, &q.Title, &q.Prompt, &tags)
	if err == sql.ErrNoRows {
		return Question{}, false, nil
	}
	if err != nil {
		return Question{}, false, err
	}
	q.Number = number
	q.Tags = splitTags(tags)
	return q, true, nil
}

func (s *Service) questionByID(ctx context.Context, questionID string, number int) (Question, bool, error) {
	var q Question
	var tags string
	err := s.db.QueryRowContext(ctx, `
SELECT question_id, title, prompt, array_to_string(topic_tags, ',')
FROM code_questions
WHERE question_id=$1`, questionID).Scan(&q.QuestionID, &q.Title, &q.Prompt, &tags)
	if err == sql.ErrNoRows {
		return Question{}, false, nil
	}
	if err != nil {
		return Question{}, false, err
	}
	q.Number = number
	q.Tags = splitTags(tags)
	return q, true, nil
}

func (s *Service) refreshSnapshot(ctx context.Context, sessionID string, source string) error {
	session, err := s.loadSession(ctx, sessionID, true)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO interview_runtime_snapshots (session_id, snapshot, source, updated_at)
VALUES ($1,$2,$3,now())
ON CONFLICT (session_id) DO UPDATE SET
  snapshot=EXCLUDED.snapshot,
  source=EXCLUDED.source,
  updated_at=now()`, sessionID, raw, source)
	return err
}

func hashAnswer(answer string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(answer)))
	return hex.EncodeToString(sum[:])
}

func questionText(q *Question) string {
	if q == nil {
		return ""
	}
	if strings.TrimSpace(q.Prompt) != "" {
		return q.Prompt
	}
	return q.Title
}

func cloneMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}

func nullEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func splitTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func floatFromMap(input map[string]any, key string) float64 {
	switch value := input[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case string:
		parsed, _ := strconv.ParseFloat(value, 64)
		return parsed
	default:
		return 0
	}
}

func boolFromMap(input map[string]any, key string) bool {
	switch value := input[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true")
	default:
		return false
	}
}

func stringFromMap(input map[string]any, key string) string {
	value, _ := input[key].(string)
	return strings.TrimSpace(value)
}
