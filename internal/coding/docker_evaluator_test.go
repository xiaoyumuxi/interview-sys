package coding

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeDockerRunner struct {
	results []commandResult
	calls   []fakeDockerCall
}

type fakeDockerCall struct {
	name  string
	args  []string
	stdin string
}

func (f *fakeDockerRunner) Run(ctx context.Context, name string, args []string, stdin string) (commandResult, error) {
	_ = ctx
	f.calls = append(f.calls, fakeDockerCall{name: name, args: args, stdin: stdin})
	if len(f.results) == 0 {
		return commandResult{}, nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func TestDockerEvaluatorAcceptsGoSubmission(t *testing.T) {
	runner := &fakeDockerRunner{results: []commandResult{
		{Stdout: "[0,1]\n"},
		{Stdout: "[1,2]\n"},
	}}
	evaluator := NewDockerEvaluator(DockerEvaluatorConfig{
		DockerBinary: "docker-test",
		GoImage:      "golang:test",
		Timeout:      time.Second,
	})
	evaluator.runner = runner

	result, err := evaluator.Evaluate(context.Background(), Submission{
		SubmissionID: "sub_1",
		Language:     "go",
		SourceCode:   "package main\nfunc main(){}",
	}, Question{QuestionID: "two-sum"}, []TestCase{
		{TestCaseID: "sample", CaseType: "sample", InputText: "case1", ExpectedOutput: "[0,1]", Weight: 1},
		{TestCaseID: "hidden", CaseType: "hidden", InputText: "case2", ExpectedOutput: "[1,2]", Weight: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusAccepted || result.Score != 100 {
		t.Fatalf("result = %+v", result)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(runner.calls))
	}
	if runner.calls[0].name != "docker-test" || runner.calls[0].stdin != "case1" {
		t.Fatalf("unexpected call = %+v", runner.calls[0])
	}
	if !strings.Contains(strings.Join(runner.calls[0].args, " "), "--network none") {
		t.Fatalf("docker args do not disable network: %#v", runner.calls[0].args)
	}
	public := result.Result["test_results"].([]map[string]any)
	if public[1]["hidden"] != true || public[1]["expected_output"] != nil {
		t.Fatalf("hidden public result leaked expected output: %#v", public[1])
	}
}

func TestDockerEvaluatorSupportsCommonLanguageRunners(t *testing.T) {
	tests := []struct {
		language string
		image    string
		file     string
		command  string
	}{
		{language: "go", image: "golang:1.26-alpine", file: "Main.go", command: "go run /work/Main.go"},
		{language: "java", image: "eclipse-temurin:21-jdk-alpine", file: "Main.java", command: "javac /work/Main.java"},
		{language: "python3", image: "python:3.13-alpine", file: "Main.py", command: "python /work/Main.py"},
		{language: "javascript", image: "node:22-alpine", file: "Main.js", command: "node /work/Main.js"},
		{language: "typescript", image: "denoland/deno:alpine-2.1.4", file: "Main.ts", command: "deno run"},
		{language: "cpp", image: "gcc:14-alpine", file: "Main.cpp", command: "g++ -std=c++20"},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			runner := &fakeDockerRunner{results: []commandResult{{Stdout: "ok\n"}}}
			evaluator := NewDockerEvaluator(DockerEvaluatorConfig{})
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
			if len(runner.calls) != 1 {
				t.Fatalf("calls = %d, want 1", len(runner.calls))
			}
			args := strings.Join(runner.calls[0].args, " ")
			if !strings.Contains(args, test.image) || !strings.Contains(args, test.command) || !strings.Contains(args, test.file) {
				t.Fatalf("docker args = %#v", runner.calls[0].args)
			}
			if !strings.Contains(args, "/work:ro") {
				t.Fatalf("docker args should mount source read-only: %#v", runner.calls[0].args)
			}
			if result.ResourceUsage["image"] != test.image {
				t.Fatalf("image = %v, want %s", result.ResourceUsage["image"], test.image)
			}
		})
	}
}

func TestDockerEvaluatorReportsWrongAnswer(t *testing.T) {
	runner := &fakeDockerRunner{results: []commandResult{{Stdout: "[1,0]\n"}}}
	evaluator := NewDockerEvaluator(DockerEvaluatorConfig{})
	evaluator.runner = runner

	result, err := evaluator.Evaluate(context.Background(), Submission{
		SubmissionID: "sub_1",
		Language:     "go",
		SourceCode:   "package main\nfunc main(){}",
	}, Question{}, []TestCase{
		{TestCaseID: "sample", CaseType: "sample", InputText: "case1", ExpectedOutput: "[0,1]", Weight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusWrongAnswer || result.Score != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestDockerEvaluatorClassifiesTimeoutAndCompileError(t *testing.T) {
	tests := []struct {
		name   string
		run    commandResult
		status string
	}{
		{name: "timeout", run: commandResult{TimedOut: true}, status: StatusTimeLimitExceeded},
		{name: "compile", run: commandResult{ExitCode: 1, Stderr: "# command-line-arguments\n./Main.go:1: syntax error"}, status: StatusCompileError},
		{name: "runtime", run: commandResult{ExitCode: 2, Stderr: "panic: boom"}, status: StatusRuntimeError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeDockerRunner{results: []commandResult{test.run}}
			evaluator := NewDockerEvaluator(DockerEvaluatorConfig{})
			evaluator.runner = runner
			result, err := evaluator.Evaluate(context.Background(), Submission{
				SubmissionID: "sub_1",
				Language:     "go",
				SourceCode:   "package main\nfunc main(){}",
			}, Question{}, []TestCase{{TestCaseID: "sample", CaseType: "sample", InputText: "case1", ExpectedOutput: "ok", Weight: 1}})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != test.status {
				t.Fatalf("status = %q, want %q, result = %+v", result.Status, test.status, result)
			}
		})
	}
}

func TestDockerEvaluatorRejectsUnsupportedLanguageAndMissingCases(t *testing.T) {
	evaluator := NewDockerEvaluator(DockerEvaluatorConfig{})
	result, err := evaluator.Evaluate(context.Background(), Submission{Language: "ruby"}, Question{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSystemError || result.Result["error_code"] != "unsupported_language_for_docker_judge" {
		t.Fatalf("unsupported language result = %+v", result)
	}
	result, err = evaluator.Evaluate(context.Background(), Submission{Language: "go"}, Question{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSystemError || result.Result["error_code"] != "test_cases_missing" {
		t.Fatalf("missing cases result = %+v", result)
	}
}
