package coding

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeWarmDockerRunner struct {
	results []warmDockerResult
	calls   []fakeDockerCall
}

type warmDockerResult struct {
	result commandResult
	err    error
}

func (f *fakeWarmDockerRunner) Run(ctx context.Context, name string, args []string, stdin string) (commandResult, error) {
	_ = ctx
	f.calls = append(f.calls, fakeDockerCall{name: name, args: args, stdin: stdin})
	if len(f.results) == 0 {
		return commandResult{}, nil
	}
	item := f.results[0]
	f.results = f.results[1:]
	return item.result, item.err
}

func TestDockerWarmEvaluatorCreatesStartsExecutesAndStopsContainer(t *testing.T) {
	runner := &fakeWarmDockerRunner{results: []warmDockerResult{
		{err: errors.New("container missing")},  // inspect
		{},                                      // create
		{},                                      // start
		{},                                      // reset
		{},                                      // cp
		{result: commandResult{Stdout: "ok\n"}}, // exec
		{},                                      // stop
	}}
	evaluator := NewDockerWarmEvaluator(DockerEvaluatorConfig{
		DockerBinary: "docker-test",
		GoImage:      "golang:test",
	}, "judge-test")
	evaluator.runner = runner

	result, err := evaluator.Evaluate(context.Background(), Submission{
		SubmissionID: "sub_1",
		Language:     "go",
		SourceCode:   "package main\nfunc main(){}",
	}, Question{}, []TestCase{{TestCaseID: "sample", CaseType: "sample", InputText: "input", ExpectedOutput: "ok", Weight: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusAccepted {
		t.Fatalf("status = %q", result.Status)
	}
	if result.ResourceUsage["sandbox"] != "docker_warm" || result.ResourceUsage["container_name"] != "judge-test-go" {
		t.Fatalf("resource usage = %#v", result.ResourceUsage)
	}
	joined := warmCallSummary(runner.calls)
	for _, want := range []string{"inspect judge-test-go", "create --name judge-test-go", "start judge-test-go", "cp ", "exec -i judge-test-go", "stop -t 1 judge-test-go"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("call summary missing %q: %s", want, joined)
		}
	}
	if !strings.Contains(joined, "--tmpfs /work:rw,exec,nosuid,size=64m") {
		t.Fatalf("warm container should use tmpfs workdir: %s", joined)
	}
}

func TestDockerWarmEvaluatorReusesExistingContainer(t *testing.T) {
	runner := &fakeWarmDockerRunner{results: []warmDockerResult{
		{}, // inspect exists
		{}, // start
		{}, // reset
		{}, // cp
		{result: commandResult{Stdout: "ok\n"}},
		{}, // stop
	}}
	evaluator := NewDockerWarmEvaluator(DockerEvaluatorConfig{}, "judge-test")
	evaluator.runner = runner

	_, err := evaluator.Evaluate(context.Background(), Submission{
		SubmissionID: "sub_1",
		Language:     "python",
		SourceCode:   "print('ok')",
	}, Question{}, []TestCase{{TestCaseID: "sample", CaseType: "sample", InputText: "input", ExpectedOutput: "ok", Weight: 1}})
	if err != nil {
		t.Fatal(err)
	}
	joined := warmCallSummary(runner.calls)
	if strings.Contains(joined, "create --name") {
		t.Fatalf("existing container should be reused, calls: %s", joined)
	}
	if !strings.Contains(joined, "start judge-test-python") {
		t.Fatalf("calls = %s", joined)
	}
}

func warmCallSummary(calls []fakeDockerCall) string {
	parts := make([]string, 0, len(calls))
	for _, call := range calls {
		parts = append(parts, strings.Join(call.args, " "))
	}
	return strings.Join(parts, " | ")
}
