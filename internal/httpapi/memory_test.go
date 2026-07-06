package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-interview-platform/internal/config"
	airuntime "ai-interview-platform/internal/runtime"
)

func TestListMemoryCandidatesForwardsUserBoundary(t *testing.T) {
	var runtimePath string
	var runtimeQuery string
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runtimePath = r.URL.Path
		runtimeQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": "runtime.memory.candidates.v1",
			"items":          []any{},
		})
	}))
	defer runtime.Close()

	router := testMemoryRouter(runtime.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/memory/candidates?user_id=user_123&status=pending&limit=500", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if runtimePath != "/api/runtime/memory/candidates" {
		t.Fatalf("runtime path = %q", runtimePath)
	}
	if !strings.Contains(runtimeQuery, "user_id=user_123") || !strings.Contains(runtimeQuery, "status=pending") || !strings.Contains(runtimeQuery, "limit=200") {
		t.Fatalf("runtime query = %q", runtimeQuery)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["schema_version"] != "memory.candidates.v1" {
		t.Fatalf("schema_version = %v", body["schema_version"])
	}
}

func TestCreateMemoryCandidateProxiesRuntime(t *testing.T) {
	var runtimeBody map[string]any
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/runtime/memory/candidates" || r.Method != http.MethodPost {
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &runtimeBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": "runtime.memory.candidate.v1",
			"item": map[string]any{
				"candidate_id": "mem_1",
				"user_id":      runtimeBody["user_id"],
				"type":         runtimeBody["type"],
				"content":      runtimeBody["content"],
			},
		})
	}))
	defer runtime.Close()

	router := testMemoryRouter(runtime.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/memory/candidates", strings.NewReader(`{
		"user_id":"user_123",
		"type":"weak_point",
		"content":"Needs more Redis Stream practice",
		"confidence":0.8
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if runtimeBody["user_id"] != "user_123" {
		t.Fatalf("runtime user_id = %v", runtimeBody["user_id"])
	}
	if runtimeBody["type"] != "weak_point" {
		t.Fatalf("runtime type = %v", runtimeBody["type"])
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["schema_version"] != "memory.candidate.v1" || body["trace_id"] == "" {
		t.Fatalf("unexpected response = %#v", body)
	}
}

func TestApproveMemoryCandidateChecksAccessBeforeMutation(t *testing.T) {
	var calls []string
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/runtime/memory/candidates":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"schema_version": "runtime.memory.candidates.v1",
				"items": []any{
					map[string]any{"candidate_id": "mem_1", "user_id": "user_123", "status": "pending"},
				},
			})
		case "/api/runtime/memory/candidates/mem_1/approve":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"schema_version": "runtime.memory.candidate.v1",
				"item":           map[string]any{"candidate_id": "mem_1", "user_id": "user_123", "status": "approved"},
			})
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtime.Close()

	router := testMemoryRouter(runtime.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/memory/candidates/mem_1/approve?user_id=user_123", strings.NewReader(`{"review_note":"looks right"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	want := []string{
		"GET /api/runtime/memory/candidates",
		"POST /api/runtime/memory/candidates/mem_1/approve",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %#v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q", i, calls[i], want[i])
		}
	}
}

func testMemoryRouter(runtimeURL string) http.Handler {
	return NewRouter(Dependencies{
		Config: config.Config{
			AuthDisabled: true,
		},
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		RuntimeClient: airuntime.NewClient(runtimeURL),
	})
}
