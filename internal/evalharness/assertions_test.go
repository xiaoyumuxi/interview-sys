package evalharness

import "testing"

func TestEvaluateAssertionsPassesConfiguredRules(t *testing.T) {
	output := map[string]any{
		"score": 88,
		"summary": map[string]any{
			"overall": "Redis single-flight and outbox recovery are explained.",
		},
		"ok": true,
	}
	expected := map[string]any{
		"required_fields": []any{"score", "summary.overall"},
		"contains": map[string]any{
			"summary.overall": "outbox",
		},
		"equals": map[string]any{
			"ok": true,
		},
	}

	assertions := EvaluateAssertions(output, expected)
	if len(assertions) != 4 {
		t.Fatalf("expected 4 assertions, got %d", len(assertions))
	}
	status, score := StatusAndScore(assertions)
	if status != "passed" {
		t.Fatalf("expected passed status, got %s", status)
	}
	if score != 100 {
		t.Fatalf("expected score 100, got %.2f", score)
	}
}

func TestEvaluateAssertionsReportsFailures(t *testing.T) {
	output := map[string]any{
		"summary": map[string]any{
			"overall": "answer missed the Redis failure mode",
		},
	}
	expected := map[string]any{
		"required_fields": []any{"score"},
		"contains": map[string]any{
			"summary.overall": "PostgreSQL snapshot",
		},
	}

	assertions := EvaluateAssertions(output, expected)
	status, score := StatusAndScore(assertions)
	if status != "failed" {
		t.Fatalf("expected failed status, got %s", status)
	}
	if score != 0 {
		t.Fatalf("expected score 0, got %.2f", score)
	}
}

func TestEvaluateAssertionsDefaultsToRuntimeOK(t *testing.T) {
	assertions := EvaluateAssertions(map[string]any{"ok": true}, map[string]any{})
	if len(assertions) != 1 {
		t.Fatalf("expected default assertion, got %d", len(assertions))
	}
	if assertions[0].Operator != "runtime_ok" || !assertions[0].Passed {
		t.Fatalf("unexpected default assertion: %#v", assertions[0])
	}
}
