package contextengine

import (
	"context"
	"testing"
)

type fakeRecentHistorySource struct {
	lastSessionID string
	lastLimit     int
	turns         []RecentTurn
	err           error
}

func (f *fakeRecentHistorySource) RecentTurns(ctx context.Context, sessionID string, limit int) ([]RecentTurn, error) {
	_ = ctx
	f.lastSessionID = sessionID
	f.lastLimit = limit
	return f.turns, f.err
}

func TestRetrieveCombinesSkillMemoryAndRecentHistory(t *testing.T) {
	registry := testSkillRegistry(t)
	memory := &fakeMemorySource{resp: map[string]any{
		"schema_version": "runtime.memory.search.v1",
		"items": []any{
			map[string]any{
				"candidate": map[string]any{
					"candidate_id": "mem_redis",
					"type":         "weak_point",
					"topic":        "redis",
					"content":      "Needs more practice explaining Redis Stream pending reclaim.",
					"confidence":   0.88,
					"status":       "approved",
				},
				"memory_context_score": 0.93,
				"reasons":              []any{"query matched redis"},
			},
		},
	}}
	recent := &fakeRecentHistorySource{turns: []RecentTurn{
		{
			TurnID:         "turn_1",
			SessionID:      "sess_1",
			QuestionID:     "q_1",
			QuestionNumber: 1,
			UserAnswer:     "Redis Streams use consumer groups and pending reclaim.",
			Evaluation:     map[string]any{"strengths": []any{"knows pending reclaim"}},
			Score:          8,
			TurnStatus:     "completed",
			CreatedAt:      "2026-07-06T10:00:00Z",
		},
	}}
	engine := New(1600, registry)
	engine.SetMemorySource(memory)
	engine.SetRecentHistorySource(recent)

	resp, err := engine.Retrieve(context.Background(), RetrievalRequest{
		TaskType:  "answer_evaluation",
		SkillID:   "java-backend",
		UserID:    "user_123",
		SessionID: "sess_1",
		Query:     "redis stream pending reclaim",
		Limit:     8,
		Debug:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.SchemaVersion != "retrieval.harness.v1" {
		t.Fatalf("schema_version = %v", resp.SchemaVersion)
	}
	if memory.lastUserID != "user_123" || memory.lastQuery != "redis stream pending reclaim" {
		t.Fatalf("unexpected memory search: user=%q query=%q", memory.lastUserID, memory.lastQuery)
	}
	if recent.lastSessionID != "sess_1" || recent.lastLimit != 8 {
		t.Fatalf("unexpected recent history search: session=%q limit=%d", recent.lastSessionID, recent.lastLimit)
	}
	if findRetrievalItem(resp.Items, "approved_memory") == nil {
		t.Fatalf("approved memory missing: %#v", resp.Items)
	}
	if findRetrievalItem(resp.Items, "recent_history") == nil {
		t.Fatalf("recent history missing: %#v", resp.Items)
	}
	if findRetrievalItem(resp.Items, "skill_reference_full_text") == nil {
		t.Fatalf("skill reference missing: %#v", resp.Items)
	}
	if resp.Debug == nil || resp.Debug.CandidateCounts["approved_memory"] != 1 || resp.Debug.CandidateCounts["recent_history"] != 1 {
		t.Fatalf("unexpected debug trace: %#v", resp.Debug)
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected vector fallback warning")
	}
}

func TestRetrieveRespectsTokenBudget(t *testing.T) {
	registry := testSkillRegistry(t)
	engine := New(60, registry)

	resp, err := engine.Retrieve(context.Background(), RetrievalRequest{
		TaskType:    "answer_evaluation",
		SkillID:     "java-backend",
		Query:       "redis stream pending reclaim",
		Limit:       10,
		TokenBudget: 20,
		Debug:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	used := 0
	for _, item := range resp.Items {
		used += item.Tokens
	}
	if used > 20 {
		t.Fatalf("used tokens = %d, budget = 20, items = %#v", used, resp.Items)
	}
	if resp.Debug == nil || resp.Debug.SkippedForBudget == 0 {
		t.Fatalf("expected budget skips in debug trace: %#v", resp.Debug)
	}
}

func findRetrievalItem(items []RetrievalItem, sourceType string) *RetrievalItem {
	for i := range items {
		if items[i].SourceType == sourceType {
			return &items[i]
		}
	}
	return nil
}
