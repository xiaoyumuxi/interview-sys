package coding

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeNativeRunner struct {
	results []commandResult
	calls   []fakeNativeCall
}

type fakeNativeCall struct {
	workDir string
	command string
	stdin   string
}

func (f *fakeNativeRunner) Run(ctx context.Context, workDir string, command string, stdin string) (commandResult, error) {
	_ = ctx
	f.calls = append(f.calls, fakeNativeCall{workDir: workDir, command: command, stdin: stdin})
	if len(f.results) == 0 {
		return commandResult{}, nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func TestNativeEvaluatorSupportsCommonLanguageRunners(t *testing.T) {
	tests := []struct {
		language string
		command  string
	}{
		{language: "go", command: "go run Main.go"},
		{language: "java", command: "javac Main.java"},
		{language: "python3", command: "python3 Main.py"},
		{language: "javascript", command: "node Main.js"},
		{language: "typescript", command: "deno run --no-prompt --no-net Main.ts"},
		{language: "cpp", command: "g++ -std=c++20"},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			runner := &fakeNativeRunner{results: []commandResult{{Stdout: "ok\n"}}}
			evaluator := NewNativeEvaluator(NativeEvaluatorConfig{Timeout: time.Second})
			evaluator.runner = runner
			result, err := evaluator.Evaluate(context.Background(), Submission{
				SubmissionID: "sub_1",
				Language:     test.language,
				SourceCode:   "// source",
			}, Question{}, []TestCase{{TestCaseID: "sample", CaseType: "sample", InputText: "input", ExpectedOutput: "ok", Weight: 1}})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != StatusAccepted {
				t.Fatalf("status = %q", result.Status)
			}
			if len(runner.calls) != 1 || !strings.Contains(runner.calls[0].command, test.command) {
				t.Fatalf("calls = %#v", runner.calls)
			}
			if result.ResourceUsage["sandbox"] != "native_trusted" {
				t.Fatalf("resource usage = %#v", result.ResourceUsage)
			}
		})
	}
}

func TestNativeEvaluatorReportsWrongAnswerAndUnsupportedLanguage(t *testing.T) {
	runner := &fakeNativeRunner{results: []commandResult{{Stdout: "no\n"}}}
	evaluator := NewNativeEvaluator(NativeEvaluatorConfig{})
	evaluator.runner = runner
	result, err := evaluator.Evaluate(context.Background(), Submission{
		SubmissionID: "sub_1",
		Language:     "python",
		SourceCode:   "print('no')",
	}, Question{}, []TestCase{{TestCaseID: "sample", CaseType: "sample", InputText: "input", ExpectedOutput: "yes", Weight: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusWrongAnswer {
		t.Fatalf("status = %q", result.Status)
	}
	result, err = evaluator.Evaluate(context.Background(), Submission{Language: "ruby"}, Question{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSystemError || result.Result["error_code"] != "unsupported_language_for_native_judge" {
		t.Fatalf("unsupported result = %+v", result)
	}
}
