package coding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"ai-interview-platform/internal/store"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

type QuestionSet struct {
	SetID        string `json:"set_id"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	Source       string `json:"source"`
	SourceURL    string `json:"source_url"`
	QuestionType string `json:"question_type"`
}

type Question struct {
	QuestionID        string   `json:"question_id"`
	SetID             string   `json:"set_id"`
	Title             string   `json:"title"`
	Difficulty        string   `json:"difficulty"`
	Source            string   `json:"source"`
	SourceURL         string   `json:"source_url"`
	QuestionType      string   `json:"question_type"`
	FrequencyRank     *int     `json:"frequency_rank,omitempty"`
	CompanyTags       []string `json:"company_tags"`
	TopicTags         []string `json:"topic_tags"`
	Prompt            string   `json:"prompt,omitempty"`
	InputFormat       string   `json:"input_format,omitempty"`
	OutputFormat      string   `json:"output_format,omitempty"`
	ConstraintsText   string   `json:"constraints_text,omitempty"`
	ReferenceSolution string   `json:"reference_solution,omitempty"`
	Explanation       string   `json:"explanation,omitempty"`
	Status            string   `json:"status"`
}

type SubmissionCreateRequest struct {
	QuestionID string `json:"question_id"`
	Language   string `json:"language"`
	SourceCode string `json:"source_code"`
}

type Submission struct {
	SubmissionID string         `json:"submission_id"`
	UserID       string         `json:"user_id,omitempty"`
	QuestionID   string         `json:"question_id"`
	Language     string         `json:"language"`
	SourceCode   string         `json:"source_code,omitempty"`
	Status       string         `json:"status"`
	Score        float64        `json:"score"`
	Result       map[string]any `json:"result"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
}

func (s *Store) ListSets(ctx context.Context) (items []QuestionSet, err error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT set_id, display_name, description, source, source_url, question_type
FROM code_question_sets
ORDER BY set_id`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	for rows.Next() {
		var item QuestionSet
		if err := rows.Scan(&item.SetID, &item.DisplayName, &item.Description, &item.Source, &item.SourceURL, &item.QuestionType); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListQuestions(ctx context.Context, questionType string, limit int) (items []Question, err error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
SELECT question_id, COALESCE(set_id,''), title, difficulty, source, source_url, question_type,
       frequency_rank, array_to_string(company_tags, ','), array_to_string(topic_tags, ','), status
FROM code_questions
WHERE status='published'`
	var args []any
	if strings.TrimSpace(questionType) != "" {
		args = append(args, questionType)
		query += ` AND question_type=$1`
	}
	args = append(args, limit)
	query += ` ORDER BY frequency_rank NULLS LAST, title LIMIT $` + strconv.Itoa(len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for rows.Next() {
		var item Question
		var rank sql.NullInt64
		var companyTags, topicTags string
		if err := rows.Scan(&item.QuestionID, &item.SetID, &item.Title, &item.Difficulty, &item.Source, &item.SourceURL, &item.QuestionType, &rank, &companyTags, &topicTags, &item.Status); err != nil {
			return nil, err
		}
		item.CompanyTags = splitTags(companyTags)
		item.TopicTags = splitTags(topicTags)
		if rank.Valid {
			item.FrequencyRank = new(int)
			*item.FrequencyRank = int(rank.Int64)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetQuestion(ctx context.Context, id string) (Question, bool, error) {
	var item Question
	var rank sql.NullInt64
	var companyTags, topicTags string
	err := s.db.QueryRowContext(ctx, `
SELECT question_id, COALESCE(set_id,''), title, difficulty, source, source_url, question_type,
       frequency_rank, array_to_string(company_tags, ','), array_to_string(topic_tags, ','), prompt, input_format, output_format,
       constraints_text, reference_solution, explanation, status
FROM code_questions
WHERE question_id=$1`, id).Scan(
		&item.QuestionID, &item.SetID, &item.Title, &item.Difficulty, &item.Source, &item.SourceURL, &item.QuestionType,
		&rank, &companyTags, &topicTags, &item.Prompt, &item.InputFormat, &item.OutputFormat,
		&item.ConstraintsText, &item.ReferenceSolution, &item.Explanation, &item.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Question{}, false, nil
	}
	if err != nil {
		return Question{}, false, err
	}
	if rank.Valid {
		item.FrequencyRank = new(int)
		*item.FrequencyRank = int(rank.Int64)
	}
	item.CompanyTags = splitTags(companyTags)
	item.TopicTags = splitTags(topicTags)
	return item, true, nil
}

func (s *Store) CreateSubmission(ctx context.Context, userID string, req SubmissionCreateRequest) (Submission, error) {
	questionID := strings.TrimSpace(req.QuestionID)
	language := normalizeLanguage(req.Language)
	sourceCode := strings.TrimSpace(req.SourceCode)
	if questionID == "" {
		return Submission{}, errors.New("question_id is required")
	}
	if language == "" {
		return Submission{}, errors.New("language is required")
	}
	if sourceCode == "" {
		return Submission{}, errors.New("source_code is required")
	}
	if len(sourceCode) > 200000 {
		return Submission{}, errors.New("source_code is too large")
	}
	if !supportedLanguage(language) {
		return Submission{}, errors.New("language is not supported")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM code_questions WHERE question_id=$1 AND status='published')`, questionID).Scan(&exists); err != nil {
		return Submission{}, err
	}
	if !exists {
		return Submission{}, errors.New("question_id is not published or does not exist")
	}
	submissionID := store.NewID("sub")
	resultJSON, _ := json.Marshal(map[string]any{"schema_version": "coding.submission.result.v1", "message": "queued for judge worker"})
	_, err := s.db.ExecContext(ctx, `
INSERT INTO code_submissions (
  submission_id, user_id, question_id, language, source_code, status, result, updated_at
) VALUES ($1,$2,$3,$4,$5,'queued',$6,now())`,
		submissionID,
		nullString(strings.TrimSpace(userID)),
		questionID,
		language,
		sourceCode,
		resultJSON,
	)
	if err != nil {
		return Submission{}, err
	}
	item, ok, err := s.GetSubmission(ctx, submissionID)
	if err != nil {
		return Submission{}, err
	}
	if !ok {
		return Submission{}, errors.New("submission was not saved")
	}
	return item, nil
}

func (s *Store) ListSubmissions(ctx context.Context, userID string, questionID string, limit int) (items []Submission, err error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
SELECT submission_id, COALESCE(user_id,''), question_id, language, source_code, status, score, result, created_at, updated_at
FROM code_submissions`
	var conditions []string
	var args []any
	if strings.TrimSpace(userID) != "" {
		args = append(args, strings.TrimSpace(userID))
		conditions = append(conditions, "user_id=$"+strconv.Itoa(len(args)))
	}
	if strings.TrimSpace(questionID) != "" {
		args = append(args, strings.TrimSpace(questionID))
		conditions = append(conditions, "question_id=$"+strconv.Itoa(len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	args = append(args, limit)
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for rows.Next() {
		item, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetSubmission(ctx context.Context, submissionID string) (Submission, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT submission_id, COALESCE(user_id,''), question_id, language, source_code, status, score, result, created_at, updated_at
FROM code_submissions
WHERE submission_id=$1`, submissionID)
	item, err := scanSubmission(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Submission{}, false, nil
	}
	if err != nil {
		return Submission{}, false, err
	}
	return item, true, nil
}

func splitTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return make([]string, 0)
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

type submissionScanner interface {
	Scan(dest ...any) error
}

func scanSubmission(row submissionScanner) (Submission, error) {
	var item Submission
	var resultBytes []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&item.SubmissionID,
		&item.UserID,
		&item.QuestionID,
		&item.Language,
		&item.SourceCode,
		&item.Status,
		&item.Score,
		&resultBytes,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Submission{}, err
	}
	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)
	item.Result = map[string]any{}
	if len(resultBytes) > 0 {
		_ = json.Unmarshal(resultBytes, &item.Result)
	}
	return item, nil
}

func normalizeLanguage(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func supportedLanguage(value string) bool {
	switch normalizeLanguage(value) {
	case "go", "java", "python", "python3", "javascript", "typescript", "cpp", "c++":
		return true
	default:
		return false
	}
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
