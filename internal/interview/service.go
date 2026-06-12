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
	CreatedAt        string         `json:"created_at"`
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
	s.queue.Publish(ctx, workqueue.Event{Type: "interview.session_created", SessionID: sessionID, Payload: map[string]any{"skill_id": req.SkillID, "question_id": question.QuestionID}})
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

	_, _ = s.db.ExecContext(ctx, `
UPDATE interview_sessions
SET session_status=$2, flow_status=$3, updated_at=now()
WHERE session_id=$1 AND flow_status <> $4`,
		sessionID, SessionInProgress, FlowEvaluating, FlowCompleted)

	flightKey := strings.Join([]string{
		"interview-evaluation",
		sessionID,
		strconv.Itoa(session.CurrentQuestionNumber),
		strconv.Itoa(session.AnswerRound),
		answerHash,
	}, "|")
	s.queue.Publish(ctx, workqueue.Event{Type: "interview.answer_submitted", SessionID: sessionID, Payload: map[string]any{"request_id": requestID, "question_number": session.CurrentQuestionNumber, "answer_round": session.AnswerRound}})

	result, err := s.flights.Execute(ctx, flightKey, func(runCtx context.Context) (string, error) {
		payload, err := s.evaluateAnswer(runCtx, session, answer, req.DryRun)
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
			return nil, errors.New("current answer is processing, please retry later")
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE interview_sessions SET session_status=$2, last_error=$3, updated_at=now() WHERE session_id=$1`, sessionID, SessionFailed, err.Error())
		return nil, err
	}
	var evaluationPayload evaluationPayload
	if err := json.Unmarshal([]byte(result.Value), &evaluationPayload); err != nil {
		return nil, err
	}
	response, err := s.persistAnswerResult(ctx, session, req, requestID, answerHash, answer, evaluationPayload, result.Replay)
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
	s.queue.Publish(ctx, workqueue.Event{Type: "interview.session_finalized", SessionID: sessionID, Payload: map[string]any{"source": "api"}})
	_ = s.refreshSnapshot(ctx, sessionID, "finalize")
	return s.GetSession(ctx, sessionID)
}

func (s *Service) Trace(ctx context.Context, sessionID string) ([]Turn, error) {
	return s.loadTurns(ctx, sessionID)
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

func (s *Service) persistAnswerResult(ctx context.Context, session Session, req SubmitAnswerRequest, requestID string, answerHash string, answer string, payload evaluationPayload, replay bool) (map[string]any, error) {
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
			return nil, err
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
	response := map[string]any{
		"schema_version":       "interview.answer.v1",
		"session_id":           session.SessionID,
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
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
INSERT INTO interview_turns (
  turn_id, session_id, question_id, question_number, answer_round, request_id, answer_hash,
  user_answer, evaluation, follow_up_needed, follow_up_question, score, trace_id, response
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		store.NewID("turn"), session.SessionID, nullEmpty(session.CurrentQuestionID), session.CurrentQuestionNumber,
		session.AnswerRound, requestID, answerHash, answer, evalJSON, followUpNeeded,
		payload.FollowUpQuestion, payload.Score, nullEmpty(payload.TraceID), responseJSON,
	)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.queue.Publish(ctx, workqueue.Event{Type: "interview.answer_evaluated", SessionID: session.SessionID, Payload: response})
	_ = s.refreshSnapshot(ctx, session.SessionID, "answer")
	return response, nil
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
       user_answer, evaluation, follow_up_needed, follow_up_question, score, COALESCE(trace_id,''), response, created_at
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
		var createdAt time.Time
		if err := rows.Scan(&turn.TurnID, &turn.SessionID, &turn.QuestionID, &turn.QuestionNumber, &turn.AnswerRound,
			&turn.RequestID, &turn.AnswerHash, &turn.UserAnswer, &evalRaw, &turn.FollowUpNeeded,
			&turn.FollowUpQuestion, &turn.Score, &turn.TraceID, &responseRaw, &createdAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(evalRaw, &turn.Evaluation)
		_ = json.Unmarshal(responseRaw, &turn.Response)
		turn.CreatedAt = createdAt.Format(time.RFC3339)
		turns = append(turns, turn)
	}
	if turns == nil {
		turns = []Turn{}
	}
	return turns, rows.Err()
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
