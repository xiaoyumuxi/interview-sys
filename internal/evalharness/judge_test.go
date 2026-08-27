package evalharness

import "testing"

func TestJudgeConfigDefaults(t *testing.T) {
	expected := map[string]any{
		"judge": map[string]any{
			"enabled": true,
			"rubric": map[string]any{
				"correctness": 40,
				"completeness": 30,
			},
		},
	}

	cfg, ok := judgeConfig(expected)
	if !ok {
		t.Fatal("expected judge config to be enabled")
	}
	if cfg.RuleWeight != defaultRuleWeight {
		t.Fatalf("rule weight = %v, want %v", cfg.RuleWeight, defaultRuleWeight)
	}
	if cfg.JudgeWeight != defaultJudgeWeight {
		t.Fatalf("judge weight = %v, want %v", cfg.JudgeWeight, defaultJudgeWeight)
	}
	if cfg.PassScore != defaultPassScore {
		t.Fatalf("pass score = %v, want %v", cfg.PassScore, defaultPassScore)
	}
	if cfg.PromptVersion != defaultJudgeVersion {
		t.Fatalf("prompt version = %q, want %q", cfg.PromptVersion, defaultJudgeVersion)
	}
}

func TestCombineScores(t *testing.T) {
	cfg := JudgeConfig{RuleWeight: 0.4, JudgeWeight: 0.6}
	got := combineScores(75, 90, cfg)
	if got != 84 {
		t.Fatalf("combined score = %v, want 84", got)
	}
}

func TestJudgeScore(t *testing.T) {
	score, ok := judgeScore(map[string]any{"total_score": 88.5})
	if !ok {
		t.Fatal("expected numeric judge score")
	}
	if score != 88.5 {
		t.Fatalf("judge score = %v, want 88.5", score)
	}
}

func TestJudgeScoreClamps(t *testing.T) {
	score, ok := judgeScore(map[string]any{"total_score": 120.0})
	if !ok {
		t.Fatal("expected numeric judge score")
	}
	if score != 100 {
		t.Fatalf("judge score = %v, want 100", score)
	}
}
