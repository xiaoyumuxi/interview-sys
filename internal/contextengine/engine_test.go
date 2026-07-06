package contextengine

import (
	"context"
	"strings"
	"testing"

	"ai-interview-platform/internal/skill"
)

type fakeMemorySource struct {
	lastUserID string
	lastQuery  string
	lastLimit  int
	resp       map[string]any
	err        error
}

func (f *fakeMemorySource) SearchMemory(ctx context.Context, userID string, query string, limit int) (map[string]any, error) {
	_ = ctx
	f.lastUserID = userID
	f.lastQuery = query
	f.lastLimit = limit
	return f.resp, f.err
}

func TestPreviewAdmitsApprovedMemory(t *testing.T) {
	registry := testSkillRegistry(t)
	memory := &fakeMemorySource{resp: map[string]any{
		"schema_version": "runtime.memory.search.v1",
		"items": []any{
			map[string]any{
				"candidate": map[string]any{
					"candidate_id": "mem_redis",
					"user_id":      "user_123",
					"type":         "weak_point",
					"topic":        "redis",
					"content":      "Needs more practice explaining Redis Stream pending reclaim.",
					"confidence":   0.82,
					"status":       "approved",
				},
				"memory_context_score": 0.91,
				"reasons":              []any{"query matched redis"},
			},
		},
	}}
	engine := New(1200, registry)
	engine.SetMemorySource(memory)

	resp, err := engine.Preview(context.Background(), PreviewRequest{
		TaskType:    "answer_evaluation",
		SkillID:     "java-backend",
		UserID:      "user_123",
		MemoryQuery: "redis stream pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	if memory.lastUserID != "user_123" || memory.lastQuery != "redis stream pending" || memory.lastLimit != memoryAdmissionLimit {
		t.Fatalf("unexpected memory search: user=%q query=%q limit=%d", memory.lastUserID, memory.lastQuery, memory.lastLimit)
	}
	if resp.MemoryAdmission == nil || !resp.MemoryAdmission.Enabled || resp.MemoryAdmission.Included != 1 {
		t.Fatalf("unexpected memory admission: %#v", resp.MemoryAdmission)
	}
	item := findContextItem(resp.Items, "memory_context")
	if item == nil {
		t.Fatal("expected memory_context item")
	}
	if item.SourceID != "mem_redis" || item.TrustLevel != "reviewed" {
		t.Fatalf("unexpected memory item: %#v", item)
	}
	if !strings.Contains(item.Content, "Redis Stream pending reclaim") || !strings.Contains(item.Reason, "query matched redis") {
		t.Fatalf("unexpected memory content/reason: %#v", item)
	}
}

func TestPreviewSkipsMemoryExtractionTask(t *testing.T) {
	registry := testSkillRegistry(t)
	memory := &fakeMemorySource{resp: map[string]any{"items": []any{}}}
	engine := New(1200, registry)
	engine.SetMemorySource(memory)

	resp, err := engine.Preview(context.Background(), PreviewRequest{
		TaskType:    "memory_extraction",
		SkillID:     "java-backend",
		UserID:      "user_123",
		MemoryQuery: "redis",
	})
	if err != nil {
		t.Fatal(err)
	}
	if memory.lastUserID != "" {
		t.Fatalf("memory search should not run, got user_id=%q", memory.lastUserID)
	}
	if resp.MemoryAdmission == nil || resp.MemoryAdmission.Enabled {
		t.Fatalf("expected disabled memory admission: %#v", resp.MemoryAdmission)
	}
	if findContextItem(resp.Items, "memory_context") != nil {
		t.Fatal("memory_context item should not be admitted for memory_extraction")
	}
}

func TestPreviewSkipsMemoryWithoutUserID(t *testing.T) {
	registry := testSkillRegistry(t)
	memory := &fakeMemorySource{resp: map[string]any{"items": []any{}}}
	engine := New(1200, registry)
	engine.SetMemorySource(memory)

	resp, err := engine.Preview(context.Background(), PreviewRequest{
		TaskType: "answer_evaluation",
		SkillID:  "java-backend",
	})
	if err != nil {
		t.Fatal(err)
	}
	if memory.lastUserID != "" {
		t.Fatalf("memory search should not run, got user_id=%q", memory.lastUserID)
	}
	if resp.MemoryAdmission == nil || len(resp.MemoryAdmission.Warnings) == 0 {
		t.Fatalf("expected missing user warning: %#v", resp.MemoryAdmission)
	}
}

func testSkillRegistry(t *testing.T) *skill.Registry {
	t.Helper()
	registry := skill.NewRegistry(t.TempDir())
	if _, err := registry.Create(skill.CreateRequest{
		ID:           "java-backend",
		DisplayName:  "Java Backend",
		Description:  "Java backend interview skill",
		Instructions: "Ask focused Java backend interview questions with evidence-based scoring.",
		Categories: []skill.Category{
			{Key: "JAVA", Label: "Java", Priority: "HIGH"},
		},
		References: map[string]string{
			"redis.md": "Redis Stream, pending reclaim, consumer groups and dead-letter handling.",
		},
	}); err != nil {
		t.Fatal(err)
	}
	return registry
}

func findContextItem(items []ContextItem, sourceType string) *ContextItem {
	for i := range items {
		if items[i].SourceType == sourceType {
			return &items[i]
		}
	}
	return nil
}
