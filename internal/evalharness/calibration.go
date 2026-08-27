package evalharness

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

const defaultCalibrationTolerance = 10.0

type CalibrationConfig struct {
	HumanScore        *float64           `json:"human_score,omitempty"`
	HumanDimensions   map[string]float64 `json:"human_dimensions,omitempty"`
	Tolerance         float64            `json:"tolerance,omitempty"`
	AnnotationVersion string             `json:"annotation_version,omitempty"`
}

type CalibrationResult struct {
	HumanScore        float64 `json:"human_score"`
	JudgeScore        float64 `json:"judge_score"`
	SignedError       float64 `json:"signed_error"`
	AbsoluteError     float64 `json:"absolute_error"`
	Tolerance         float64 `json:"tolerance"`
	WithinTolerance   bool    `json:"within_tolerance"`
	AnnotationVersion string  `json:"annotation_version,omitempty"`
	PromptVersion     string  `json:"prompt_version,omitempty"`
	RubricVersion     string  `json:"rubric_version,omitempty"`
}

type CalibrationSample struct {
	CaseID            string  `json:"case_id"`
	RunID             string  `json:"run_id"`
	HumanScore        float64 `json:"human_score"`
	JudgeScore        float64 `json:"judge_score"`
	SignedError       float64 `json:"signed_error"`
	AbsoluteError     float64 `json:"absolute_error"`
	Tolerance         float64 `json:"tolerance"`
	WithinTolerance   bool    `json:"within_tolerance"`
	AnnotationVersion string  `json:"annotation_version,omitempty"`
	PromptVersion     string  `json:"prompt_version,omitempty"`
	RubricVersion     string  `json:"rubric_version,omitempty"`
}

type CalibrationSummary struct {
	SchemaVersion       string              `json:"schema_version"`
	Suite               string              `json:"suite,omitempty"`
	TaskType            string              `json:"task_type,omitempty"`
	SampleCount         int                 `json:"sample_count"`
	MeanAbsoluteError   float64             `json:"mean_absolute_error"`
	MeanSignedError     float64             `json:"mean_signed_error"`
	WithinToleranceRate float64             `json:"within_tolerance_rate"`
	Items               []CalibrationSample `json:"items"`
}

func calibrationConfig(expected map[string]any) (CalibrationConfig, bool) {
	raw, ok := expected["calibration"]
	if !ok || raw == nil {
		return CalibrationConfig{}, false
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return CalibrationConfig{}, false
	}
	var cfg CalibrationConfig
	if err := json.Unmarshal(payload, &cfg); err != nil || cfg.HumanScore == nil {
		return CalibrationConfig{}, false
	}
	if cfg.Tolerance <= 0 {
		cfg.Tolerance = defaultCalibrationTolerance
	}
	return cfg, true
}

func validateCalibrationConfiguration(expected map[string]any) error {
	cfg, enabled := calibrationConfig(expected)
	if !enabled {
		return nil
	}
	if *cfg.HumanScore < 0 || *cfg.HumanScore > 100 {
		return fmt.Errorf("calibration.human_score must be between 0 and 100")
	}
	if cfg.Tolerance <= 0 || cfg.Tolerance > 100 {
		return fmt.Errorf("calibration.tolerance must be between 0 and 100")
	}
	if _, judgeEnabled := judgeConfig(expected); !judgeEnabled {
		return fmt.Errorf("calibration requires judge.enabled=true")
	}
	return nil
}

func calibrationResult(expected map[string]any, judgeScore float64, judgeCfg JudgeConfig) (CalibrationResult, bool) {
	cfg, ok := calibrationConfig(expected)
	if !ok {
		return CalibrationResult{}, false
	}
	humanScore := clampScore(*cfg.HumanScore)
	judgeScore = clampScore(judgeScore)
	signedError := judgeScore - humanScore
	absoluteError := math.Abs(signedError)
	return CalibrationResult{
		HumanScore:        humanScore,
		JudgeScore:        judgeScore,
		SignedError:       signedError,
		AbsoluteError:     absoluteError,
		Tolerance:         cfg.Tolerance,
		WithinTolerance:   absoluteError <= cfg.Tolerance,
		AnnotationVersion: cfg.AnnotationVersion,
		PromptVersion:     judgeCfg.PromptVersion,
		RubricVersion:     judgeCfg.RubricVersion,
	}, true
}

func (s *Service) CalibrationSummary(ctx context.Context, suite string, taskType string, limit int) (CalibrationSummary, error) {
	cases, err := s.ListCases(ctx, suite, taskType, limit)
	if err != nil {
		return CalibrationSummary{}, err
	}

	items := make([]CalibrationSample, 0, len(cases))
	for _, item := range cases {
		if _, ok := calibrationConfig(item.Expected); !ok {
			continue
		}
		runs, err := s.ListRuns(ctx, item.CaseID, item.TaskType, 20)
		if err != nil {
			return CalibrationSummary{}, err
		}
		for _, run := range runs {
			sample, ok := calibrationSampleFromRun(item.CaseID, run)
			if !ok {
				continue
			}
			items = append(items, sample)
			break
		}
	}

	return summarizeCalibration(suite, taskType, items), nil
}

func calibrationSampleFromRun(caseID string, run Run) (CalibrationSample, bool) {
	raw, ok := mapFromAny(run.Output["calibration"])
	if !ok {
		return CalibrationSample{}, false
	}
	humanScore, ok := numericValue(raw["human_score"])
	if !ok {
		return CalibrationSample{}, false
	}
	judgeScore, ok := numericValue(raw["judge_score"])
	if !ok {
		return CalibrationSample{}, false
	}
	signedError, ok := numericValue(raw["signed_error"])
	if !ok {
		return CalibrationSample{}, false
	}
	absoluteError, ok := numericValue(raw["absolute_error"])
	if !ok {
		return CalibrationSample{}, false
	}
	tolerance, ok := numericValue(raw["tolerance"])
	if !ok {
		return CalibrationSample{}, false
	}
	withinTolerance, _ := raw["within_tolerance"].(bool)
	return CalibrationSample{
		CaseID:            caseID,
		RunID:             run.RunID,
		HumanScore:        humanScore,
		JudgeScore:        judgeScore,
		SignedError:       signedError,
		AbsoluteError:     absoluteError,
		Tolerance:         tolerance,
		WithinTolerance:   withinTolerance,
		AnnotationVersion: stringValue(raw["annotation_version"]),
		PromptVersion:     stringValue(raw["prompt_version"]),
		RubricVersion:     stringValue(raw["rubric_version"]),
	}, true
}

func summarizeCalibration(suite string, taskType string, items []CalibrationSample) CalibrationSummary {
	summary := CalibrationSummary{
		SchemaVersion: "evaluation.calibration.summary.v1",
		Suite:         suite,
		TaskType:      taskType,
		SampleCount:   len(items),
		Items:         items,
	}
	if len(items) == 0 {
		return summary
	}

	var absoluteTotal float64
	var signedTotal float64
	within := 0
	for _, item := range items {
		absoluteTotal += item.AbsoluteError
		signedTotal += item.SignedError
		if item.WithinTolerance {
			within++
		}
	}
	summary.MeanAbsoluteError = absoluteTotal / float64(len(items))
	summary.MeanSignedError = signedTotal / float64(len(items))
	summary.WithinToleranceRate = float64(within) / float64(len(items))
	return summary
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		value, err := typed.Float64()
		return value, err == nil
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
