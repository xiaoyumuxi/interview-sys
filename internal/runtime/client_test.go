package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunTaskSendsOutputSchema(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/runtime/tasks" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": "runtime.task.v1",
			"task_type":      "summary",
			"ok":             true,
			"output":         map[string]any{"summary": "ok"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.RunTask(context.Background(), TaskRequest{
		TaskType:     "summary",
		ContextItems: nil,
		UserInput:    "facts",
		OutputSchema: map[string]any{"schema_version": "interview.report.content.v1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("resp.OK = false")
	}
	outputSchema, ok := body["output_schema"].(map[string]any)
	if !ok {
		t.Fatalf("output_schema missing from request: %#v", body)
	}
	if outputSchema["schema_version"] != "interview.report.content.v1" {
		t.Fatalf("schema_version = %v", outputSchema["schema_version"])
	}
}
