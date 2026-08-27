package evalharness

import "testing"

func TestCalibrationResult(t *testing.T) {
	human := 80.0
	expected := map[string]any{
		"calibration": map[string]any{
			"human_score":        human,
			"tolerance":          8,
			"annotation_version": "golden.v1",
		},
	}
	judgeCfg := JudgeConfig{PromptVersion: "judge.v2", RubricVersion: "redis.v1"}

	result, ok := calibrationResult(expected, 86, judgeCfg)
	if !ok {
		t.Fatal("expected calibration result")
	}
	if result.SignedError != 6 || result.AbsoluteError != 6 {
		t.Fatalf("unexpected calibration error: %+v", result)
	}
	if !result.WithinTolerance {
		t.Fatal("expected score to be within tolerance")
	}
}

func TestCalibrationConfigDefaultTolerance(t *testing.T) {
	human := 70.0
	cfg, ok := calibrationConfig(map[string]any{
		"calibration": map[string]any{"human_score": human},
	})
	if !ok {
		t.Fatal("expected calibration config")
	}
	if cfg.Tolerance != defaultCalibrationTolerance {
		t.Fatalf("tolerance = %v, want %v", cfg.Tolerance, defaultCalibrationTolerance)
	}
}

func TestSummarizeCalibration(t *testing.T) {
	summary := summarizeCalibration("golden", "answer_evaluation", []CalibrationSample{
		{SignedError: 4, AbsoluteError: 4, WithinTolerance: true},
		{SignedError: -10, AbsoluteError: 10, WithinTolerance: false},
	})
	if summary.SampleCount != 2 {
		t.Fatalf("sample count = %d, want 2", summary.SampleCount)
	}
	if summary.MeanAbsoluteError != 7 {
		t.Fatalf("MAE = %v, want 7", summary.MeanAbsoluteError)
	}
	if summary.MeanSignedError != -3 {
		t.Fatalf("mean signed error = %v, want -3", summary.MeanSignedError)
	}
	if summary.WithinToleranceRate != 0.5 {
		t.Fatalf("within tolerance rate = %v, want 0.5", summary.WithinToleranceRate)
	}
}
