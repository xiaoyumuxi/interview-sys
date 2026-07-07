package evalharness

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	corestore "ai-interview-platform/internal/store"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

type Case struct {
	CaseID    string         `json:"case_id"`
	Suite     string         `json:"suite"`
	TaskType  string         `json:"task_type"`
	SkillID   string         `json:"skill_id,omitempty"`
	Input     map[string]any `json:"input"`
	Expected  map[string]any `json:"expected"`
	Tags      []string       `json:"tags"`
	Status    string         `json:"status"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type Run struct {
	RunID      string         `json:"run_id"`
	CaseID     string         `json:"case_id"`
	TaskType   string         `json:"task_type"`
	Status     string         `json:"status"`
	Score      float64        `json:"score"`
	Input      map[string]any `json:"input"`
	Output     map[string]any `json:"output"`
	Assertions []Assertion    `json:"assertions"`
	TraceID    string         `json:"trace_id,omitempty"`
	ErrorText  string         `json:"error_text,omitempty"`
	DurationMS int            `json:"duration_ms"`
	CreatedAt  string         `json:"created_at"`
}

type SaveCaseRequest struct {
	CaseID   string         `json:"case_id"`
	Suite    string         `json:"suite"`
	TaskType string         `json:"task_type"`
	SkillID  string         `json:"skill_id"`
	Input    map[string]any `json:"input"`
	Expected map[string]any `json:"expected"`
	Tags     []string       `json:"tags"`
	Status   string         `json:"status"`
}

type RecordRunRequest struct {
	RunID      string         `json:"run_id"`
	CaseID     string         `json:"case_id"`
	TaskType   string         `json:"task_type"`
	Status     string         `json:"status"`
	Score      float64        `json:"score"`
	Input      map[string]any `json:"input"`
	Output     map[string]any `json:"output"`
	Assertions []Assertion    `json:"assertions"`
	TraceID    string         `json:"trace_id"`
	ErrorText  string         `json:"error_text"`
	DurationMS int            `json:"duration_ms"`
}

func (s *Store) SaveCase(ctx context.Context, req SaveCaseRequest) (Case, error) {
	req.Suite = defaultString(req.Suite, "default")
	req.TaskType = strings.TrimSpace(req.TaskType)
	req.Status = defaultString(req.Status, "active")
	if req.CaseID == "" {
		req.CaseID = corestore.NewID("eval_case")
	}
	if req.TaskType == "" {
		return Case{}, errors.New("task_type is required")
	}
	if req.Status != "active" && req.Status != "archived" {
		return Case{}, errors.New("status must be active or archived")
	}
	input, err := marshalObject(req.Input)
	if err != nil {
		return Case{}, err
	}
	expected, err := marshalObject(req.Expected)
	if err != nil {
		return Case{}, err
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO evaluation_cases (case_id, suite, task_type, skill_id, input, expected, tags, status, updated_at)
VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7::text[],$8,now())
ON CONFLICT (case_id) DO UPDATE SET
  suite = EXCLUDED.suite,
  task_type = EXCLUDED.task_type,
  skill_id = EXCLUDED.skill_id,
  input = EXCLUDED.input,
  expected = EXCLUDED.expected,
  tags = EXCLUDED.tags,
  status = EXCLUDED.status,
  updated_at = now()
RETURNING case_id, suite, task_type, COALESCE(skill_id,''), input, expected, array_to_string(tags, ','), status, created_at, updated_at`,
		req.CaseID, req.Suite, req.TaskType, strings.TrimSpace(req.SkillID), input, expected, postgresTextArray(req.Tags), req.Status,
	)
	return scanCase(row)
}

func (s *Store) GetCase(ctx context.Context, caseID string) (Case, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT case_id, suite, task_type, COALESCE(skill_id,''), input, expected, array_to_string(tags, ','), status, created_at, updated_at
FROM evaluation_cases
WHERE case_id=$1`, caseID)
	item, err := scanCase(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Case{}, false, nil
		}
		return Case{}, false, err
	}
	return item, true, nil
}

func (s *Store) ListCases(ctx context.Context, suite string, taskType string, limit int) (items []Case, err error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
SELECT case_id, suite, task_type, COALESCE(skill_id,''), input, expected, array_to_string(tags, ','), status, created_at, updated_at
FROM evaluation_cases
WHERE status='active'`
	args := []any{}
	if strings.TrimSpace(suite) != "" {
		args = append(args, suite)
		query += ` AND suite=$` + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(taskType) != "" {
		args = append(args, taskType)
		query += ` AND task_type=$` + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	query += ` ORDER BY suite, task_type, case_id LIMIT $` + strconv.Itoa(len(args))

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
		item, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RecordRun(ctx context.Context, req RecordRunRequest) (Run, error) {
	if req.RunID == "" {
		req.RunID = corestore.NewID("eval_run")
	}
	if req.CaseID == "" {
		return Run{}, errors.New("case_id is required")
	}
	if req.TaskType == "" {
		return Run{}, errors.New("task_type is required")
	}
	if req.Status != "passed" && req.Status != "failed" && req.Status != "error" {
		return Run{}, errors.New("status must be passed, failed, or error")
	}
	input, err := marshalObject(req.Input)
	if err != nil {
		return Run{}, err
	}
	output, err := marshalObject(req.Output)
	if err != nil {
		return Run{}, err
	}
	assertions, err := json.Marshal(req.Assertions)
	if err != nil {
		return Run{}, err
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO evaluation_runs (
  run_id, case_id, task_type, status, score, input, output, assertions, trace_id, error_text, duration_ms
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,$11)
RETURNING run_id, case_id, task_type, status, score, input, output, assertions, COALESCE(trace_id,''), error_text, duration_ms, created_at`,
		req.RunID, req.CaseID, req.TaskType, req.Status, req.Score, input, output, assertions, req.TraceID, req.ErrorText, req.DurationMS,
	)
	return scanRun(row)
}

func (s *Store) ListRuns(ctx context.Context, caseID string, taskType string, limit int) (items []Run, err error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
SELECT run_id, case_id, task_type, status, score, input, output, assertions, COALESCE(trace_id,''), error_text, duration_ms, created_at
FROM evaluation_runs
WHERE 1=1`
	args := []any{}
	if strings.TrimSpace(caseID) != "" {
		args = append(args, caseID)
		query += ` AND case_id=$` + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(taskType) != "" {
		args = append(args, taskType)
		query += ` AND task_type=$` + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(len(args))

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
		item, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type caseScanner interface {
	Scan(dest ...any) error
}

func scanCase(row caseScanner) (Case, error) {
	var item Case
	var inputRaw, expectedRaw []byte
	var tags string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&item.CaseID, &item.Suite, &item.TaskType, &item.SkillID, &inputRaw, &expectedRaw, &tags, &item.Status, &createdAt, &updatedAt); err != nil {
		return Case{}, err
	}
	input, err := decodeMap(inputRaw)
	if err != nil {
		return Case{}, err
	}
	expected, err := decodeMap(expectedRaw)
	if err != nil {
		return Case{}, err
	}
	item.Input = input
	item.Expected = expected
	item.Tags = splitTags(tags)
	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)
	return item, nil
}

func scanRun(row caseScanner) (Run, error) {
	var item Run
	var inputRaw, outputRaw, assertionsRaw []byte
	var createdAt time.Time
	if err := row.Scan(&item.RunID, &item.CaseID, &item.TaskType, &item.Status, &item.Score, &inputRaw, &outputRaw, &assertionsRaw, &item.TraceID, &item.ErrorText, &item.DurationMS, &createdAt); err != nil {
		return Run{}, err
	}
	input, err := decodeMap(inputRaw)
	if err != nil {
		return Run{}, err
	}
	output, err := decodeMap(outputRaw)
	if err != nil {
		return Run{}, err
	}
	var assertions []Assertion
	if len(assertionsRaw) > 0 {
		if err := json.Unmarshal(assertionsRaw, &assertions); err != nil {
			return Run{}, err
		}
	}
	item.Input = input
	item.Output = output
	item.Assertions = assertions
	item.CreatedAt = createdAt.Format(time.RFC3339)
	return item, nil
}

func marshalObject(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return json.Marshal(value)
}

func decodeMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func splitTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if tag := strings.TrimSpace(part); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func mapFromAny(value any) (map[string]any, bool) {
	out, ok := value.(map[string]any)
	return out, ok
}

func compactJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(payload)
}

func postgresTextArray(values []string) string {
	if len(values) == 0 {
		return "{}"
	}
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ReplaceAll(value, `\`, `\\`)
		value = strings.ReplaceAll(value, `"`, `\"`)
		escaped = append(escaped, `"`+value+`"`)
	}
	return "{" + strings.Join(escaped, ",") + "}"
}
