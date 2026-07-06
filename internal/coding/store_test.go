package coding

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSupportedLanguageNormalizesInput(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: " Go ", want: true},
		{value: "PYTHON3", want: true},
		{value: "c++", want: true},
		{value: "ruby", want: false},
		{value: "", want: false},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := supportedLanguage(test.value); got != test.want {
				t.Fatalf("supportedLanguage(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestNormalizeLanguage(t *testing.T) {
	if got := normalizeLanguage(" TypeScript "); got != "typescript" {
		t.Fatalf("normalizeLanguage() = %q, want %q", got, "typescript")
	}
}

func TestCreateSubmissionIntegration(t *testing.T) {
	db := openIntegrationDB(t)
	store := NewStore(db)
	ctx := context.Background()
	suffix := time.Now().Format("20060102150405.000000000")
	suffix = strings.ReplaceAll(suffix, ".", "")
	userID := "test_user_" + suffix
	setID := "test_set_" + suffix
	questionID := "test_question_" + suffix
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM code_submissions WHERE user_id=$1 OR question_id=$2`, userID, questionID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM code_questions WHERE question_id=$1`, questionID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM code_question_sets WHERE set_id=$1`, setID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM app_users WHERE user_id=$1`, userID)
	})
	seedCodingFixture(t, db, userID, setID, questionID, "published")

	item, err := store.CreateSubmission(ctx, userID, SubmissionCreateRequest{
		QuestionID: questionID,
		Language:   " Go ",
		SourceCode: "package main\nfunc main() {}",
	})
	if err != nil {
		t.Fatalf("CreateSubmission() error = %v", err)
	}
	if item.SubmissionID == "" || item.UserID != userID || item.QuestionID != questionID {
		t.Fatalf("unexpected submission identity: %+v", item)
	}
	if item.Language != "go" || item.Status != "queued" {
		t.Fatalf("unexpected submission state: language=%q status=%q", item.Language, item.Status)
	}
	if item.Result["schema_version"] != "coding.submission.result.v1" {
		t.Fatalf("result schema_version = %v", item.Result["schema_version"])
	}

	got, ok, err := store.GetSubmission(ctx, item.SubmissionID)
	if err != nil || !ok {
		t.Fatalf("GetSubmission() ok=%v error=%v", ok, err)
	}
	if got.SourceCode == "" {
		t.Fatal("GetSubmission() should include source_code for detail view")
	}

	items, err := store.ListSubmissions(ctx, userID, questionID, 10)
	if err != nil {
		t.Fatalf("ListSubmissions() error = %v", err)
	}
	if len(items) != 1 || items[0].SubmissionID != item.SubmissionID {
		t.Fatalf("ListSubmissions() = %+v, want one created submission", items)
	}

	claimed, err := store.ClaimQueuedSubmissions(ctx, 5)
	if err != nil {
		t.Fatalf("ClaimQueuedSubmissions() error = %v", err)
	}
	if len(claimed) != 1 || claimed[0].SubmissionID != item.SubmissionID || claimed[0].Status != StatusRunning {
		t.Fatalf("ClaimQueuedSubmissions() = %+v, want running created submission", claimed)
	}
	completed, err := store.CompleteSubmission(ctx, item.SubmissionID, JudgeResult{
		Status: StatusAccepted,
		Score:  100,
		Result: map[string]any{
			"message": "all tests passed",
		},
		TestResults: []map[string]any{
			{"case": "sample", "passed": true},
		},
		StdoutText:    "ok",
		ResourceUsage: map[string]any{"time_ms": 1},
	})
	if err != nil {
		t.Fatalf("CompleteSubmission() error = %v", err)
	}
	if completed.Status != StatusAccepted || completed.Score != 100 {
		t.Fatalf("completed submission = %+v", completed)
	}
	if completed.Result["judge_status"] != StatusAccepted {
		t.Fatalf("judge_status = %v", completed.Result["judge_status"])
	}
	var traceCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM code_evaluation_traces WHERE submission_id=$1`, item.SubmissionID).Scan(&traceCount); err != nil {
		t.Fatalf("count traces: %v", err)
	}
	if traceCount != 1 {
		t.Fatalf("trace count = %d, want 1", traceCount)
	}
	summary, err := store.JudgeSummary(ctx)
	if err != nil {
		t.Fatalf("JudgeSummary() error = %v", err)
	}
	if summary.ByStatus[StatusAccepted] == 0 || summary.Total == 0 {
		t.Fatalf("JudgeSummary() = %+v", summary)
	}
}

func TestCreateSubmissionRejectsUnpublishedQuestionIntegration(t *testing.T) {
	db := openIntegrationDB(t)
	store := NewStore(db)
	ctx := context.Background()
	suffix := time.Now().Format("20060102150405.000000000")
	suffix = strings.ReplaceAll(suffix, ".", "")
	userID := "test_user_" + suffix
	setID := "test_set_" + suffix
	questionID := "test_question_" + suffix
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM code_submissions WHERE user_id=$1 OR question_id=$2`, userID, questionID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM code_questions WHERE question_id=$1`, questionID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM code_question_sets WHERE set_id=$1`, setID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM app_users WHERE user_id=$1`, userID)
	})
	seedCodingFixture(t, db, userID, setID, questionID, "draft")

	_, err := store.CreateSubmission(ctx, userID, SubmissionCreateRequest{
		QuestionID: questionID,
		Language:   "java",
		SourceCode: "class Main {}",
	})
	if err == nil || !strings.Contains(err.Error(), "not published") {
		t.Fatalf("CreateSubmission() error = %v, want unpublished question error", err)
	}
}

func TestSandboxUnavailableEvaluatorDoesNotExecuteCode(t *testing.T) {
	result, err := SandboxUnavailableEvaluator{}.Evaluate(context.Background(), Submission{
		SubmissionID: "sub_1",
		Language:     "go",
		SourceCode:   "panic(\"should not run\")",
	}, Question{QuestionID: "q_1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSystemError {
		t.Fatalf("status = %q", result.Status)
	}
	if result.Result["error_code"] != "sandbox_not_configured" {
		t.Fatalf("result = %#v", result.Result)
	}
	if result.ResourceUsage["sandbox"] != "unavailable" {
		t.Fatalf("resource usage = %#v", result.ResourceUsage)
	}
}

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = readEnvFileValue(".env", "DATABASE_URL")
	}
	if dsn == "" {
		dsn = readEnvFileValue("../../.env", "DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL or DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("PostgreSQL integration database unavailable: %v", err)
	}
	return db
}

func seedCodingFixture(t *testing.T, db *sql.DB, userID string, setID string, questionID string, status string) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
INSERT INTO app_users (user_id, display_name, email, password_hash, role, status)
VALUES ($1, 'Test User', NULL, '', 'user', 'active')`, userID); err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO code_question_sets (set_id, display_name, description, source, question_type)
VALUES ($1, 'Test Set', 'integration test set', 'test', 'algorithm')`, setID); err != nil {
		t.Fatalf("insert test question set: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO code_questions (
  question_id, set_id, title, difficulty, source, question_type,
  topic_tags, prompt, status
) VALUES ($1,$2,'Integration Question','easy','test','algorithm',ARRAY['integration'],'Prompt',$3)`,
		questionID, setID, status); err != nil {
		t.Fatalf("insert test question: %v", err)
	}
}

func readEnvFileValue(path string, key string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(strings.TrimPrefix(line, prefix), "\"'")
		}
	}
	return ""
}
