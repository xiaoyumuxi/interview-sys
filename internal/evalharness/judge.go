package evalharness

import (
	"encoding/json"
	"fmt"
)

const (
	judgeTaskType       = "evaluation_judge"
	defaultRuleWeight   = 0.4
	defaultJudgeWeight  = 0.6
	defaultJudgeVersion = "judge.v1"
	defaultPassScore    = 70.0
)

type JudgeConfig struct {
	Enabled         bool           `json:"enabled"`
	ReferenceAnswer any            `json:"reference_answer,omitempty"`
	KeyPoints       []string       `json:"key_points,omitempty"`
	CommonErrors    []string       `json:"common_errors,omitempty"`
	Rubric          map[string]any `json:"rubric,omitempty"`
	RuleWeight      float64        `json:"rule_weight,omitempty"`
	JudgeWeight     float64        `json:"judge_weight,omitempty"`
	PassScore       float64        `json:"pass_score,omitempty"`
	PromptVersion   string         `json:"prompt_version,omitempty"`
	RubricVersion   string         `json:"rubric_version,omitempty"`
}

func judgeConfig(expected map[string]any) (JudgeConfig, bool) {
	raw, ok := expected["judge"]
	if !ok || raw == nil {
		return JudgeConfig{}, false
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return JudgeConfig{}, false
	}
	var cfg JudgeConfig
	if err := json.Unmarshal(payload, &cfg); err != nil || !cfg.Enabled {
		return JudgeConfig{}, false
	}
	if cfg.RuleWeight <= 0 && cfg.JudgeWeight <= 0 {
		cfg.RuleWeight = defaultRuleWeight
		cfg.JudgeWeight = defaultJudgeWeight
	}
	if cfg.PassScore <= 0 {
		cfg.PassScore = defaultPassScore
	}
	if cfg.PromptVersion == "" {
		cfg.PromptVersion = defaultJudgeVersion
	}
	return cfg, true
}

func buildJudgeInput(item Case, userInput string, generated map[string]any, cfg JudgeConfig) string {
	payload := map[string]any{
		"case_id":           item.CaseID,
		"task_type":         item.TaskType,
		"question_or_input": userInput,
		"generated_output":  generated,
		"reference_answer":  cfg.ReferenceAnswer,
		"key_points":        cfg.KeyPoints,
		"common_errors":     cfg.CommonErrors,
		"rubric":            cfg.Rubric,
		"prompt_version":    cfg.PromptVersion,
		"rubric_version":    cfg.RubricVersion,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("%v", payload)
	}
	return string(encoded)
}

func judgeOutputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"total_score", "dimensions", "summary"},
		"properties": map[string]any{
			"total_score": map[string]any{"type": "number", "minimum": 0, "maximum": 100},
			"dimensions":  map[string]any{"type": "object"},
			"summary":     map[string]any{"type": "string"},
			"fatal_error": map[string]any{"type": "boolean"},
		},
	}
}

func judgeScore(output map[string]any) (float64, bool) {
	value, ok := output["total_score"]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return clampScore(typed), true
	case float32:
		return clampScore(float64(typed)), true
	case int:
		return clampScore(float64(typed)), true
	case int64:
		return clampScore(float64(typed)), true
	case json.Number:
		n, err := typed.Float64()
		return clampScore(n), err == nil
	default:
		return 0, false
	}
}

func combineScores(ruleScore float64, llmScore float64, cfg JudgeConfig) float64 {
	ruleWeight := cfg.RuleWeight
	judgeWeight := cfg.JudgeWeight
	if ruleWeight < 0 {
		ruleWeight = 0
	}
	if judgeWeight < 0 {
		judgeWeight = 0
	}
	total := ruleWeight + judgeWeight
	if total == 0 {
		return clampScore(ruleScore)
	}
	return clampScore((ruleScore*ruleWeight + llmScore*judgeWeight) / total)
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
