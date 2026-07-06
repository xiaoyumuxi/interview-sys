package coding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DockerWarmEvaluator struct {
	cfg    DockerEvaluatorConfig
	runner dockerCommandRunner
	prefix string
}

func NewDockerWarmEvaluator(cfg DockerEvaluatorConfig, containerPrefix string) *DockerWarmEvaluator {
	base := NewDockerEvaluator(cfg)
	if strings.TrimSpace(containerPrefix) == "" {
		containerPrefix = "ai-interview-judge"
	}
	return &DockerWarmEvaluator{
		cfg:    base.cfg,
		runner: base.runner,
		prefix: strings.TrimSpace(containerPrefix),
	}
}

func (e *DockerWarmEvaluator) Evaluate(ctx context.Context, submission Submission, question Question, cases []TestCase) (JudgeResult, error) {
	_ = question
	base := &DockerEvaluator{cfg: e.cfg, runner: e.runner}
	spec, ok := base.runnerSpec(submission.Language)
	if !ok {
		return JudgeResult{
			Status: StatusSystemError,
			Result: map[string]any{
				"error_code": "unsupported_language_for_docker_judge",
				"message":    "docker warm judge does not support this language",
				"retryable":  false,
				"language":   submission.Language,
			},
		}, nil
	}
	if len(cases) == 0 {
		return JudgeResult{
			Status: StatusSystemError,
			Result: map[string]any{
				"error_code": "test_cases_missing",
				"message":    "question has no test cases",
				"retryable":  false,
			},
		}, nil
	}
	containerName := e.containerName(spec.Language)
	if err := e.ensureContainer(ctx, containerName, spec); err != nil {
		return JudgeResult{}, err
	}
	if err := e.runLifecycle(ctx, "start", containerName); err != nil {
		return JudgeResult{}, err
	}
	defer func() { _ = e.runLifecycle(context.Background(), "stop", "-t", "1", containerName) }()
	_, _ = e.runner.Run(ctx, e.cfg.DockerBinary, []string{"exec", containerName, "sh", "-c", "rm -rf /work/* /tmp/*"}, "")

	dir, err := os.MkdirTemp(e.cfg.WorkDir, "coding-warm-judge-*")
	if err != nil {
		return JudgeResult{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	sourcePath := filepath.Join(dir, spec.FileName)
	if err := os.WriteFile(sourcePath, []byte(submission.SourceCode), 0o600); err != nil {
		return JudgeResult{}, err
	}
	if err := e.runLifecycle(ctx, "cp", sourcePath, containerName+":/work/"+spec.FileName); err != nil {
		return JudgeResult{}, err
	}

	fullResults := make([]map[string]any, 0, len(cases))
	publicResults := make([]map[string]any, 0, len(cases))
	totalWeight := 0
	passedWeight := 0
	var stdoutCombined strings.Builder
	var stderrCombined strings.Builder
	for index, testCase := range cases {
		if testCase.Weight <= 0 {
			testCase.Weight = 1
		}
		totalWeight += testCase.Weight
		runCtx, cancel := context.WithTimeout(ctx, e.cfg.Timeout)
		result, err := e.runner.Run(runCtx, e.cfg.DockerBinary, []string{"exec", "-i", containerName, "sh", "-c", spec.Command}, testCase.InputText)
		cancel()
		if err != nil && !result.TimedOut && result.ExitCode == 0 {
			return JudgeResult{}, err
		}
		stdoutCombined.WriteString(result.Stdout)
		stderrCombined.WriteString(result.Stderr)
		caseResult := map[string]any{
			"test_case_id":    testCase.TestCaseID,
			"case_type":       testCase.CaseType,
			"input_text":      testCase.InputText,
			"expected_output": testCase.ExpectedOutput,
			"stdout":          strings.TrimSpace(result.Stdout),
			"stderr":          strings.TrimSpace(result.Stderr),
			"exit_code":       result.ExitCode,
			"timed_out":       result.TimedOut,
			"weight":          testCase.Weight,
		}
		if result.TimedOut {
			caseResult["passed"] = false
			fullResults = append(fullResults, caseResult)
			publicResults = append(publicResults, publicCaseResult(index, caseResult))
			return e.finalResult(StatusTimeLimitExceeded, spec, containerName, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), "time limit exceeded")
		}
		if result.ExitCode != 0 {
			caseResult["passed"] = false
			fullResults = append(fullResults, caseResult)
			publicResults = append(publicResults, publicCaseResult(index, caseResult))
			status := StatusRuntimeError
			if looksLikeCompileError(spec.Language, result.Stderr) {
				status = StatusCompileError
			}
			return e.finalResult(status, spec, containerName, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), "program exited with non-zero status")
		}
		passed := normalizeOutput(result.Stdout) == normalizeOutput(testCase.ExpectedOutput)
		caseResult["passed"] = passed
		if passed {
			passedWeight += testCase.Weight
		}
		fullResults = append(fullResults, caseResult)
		publicResults = append(publicResults, publicCaseResult(index, caseResult))
	}
	status := StatusAccepted
	message := "all test cases passed"
	if passedWeight < totalWeight {
		status = StatusWrongAnswer
		message = "one or more test cases failed"
	}
	return e.finalResult(status, spec, containerName, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), message)
}

func (e *DockerWarmEvaluator) ensureContainer(ctx context.Context, name string, spec dockerRunnerSpec) error {
	inspectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err := e.runner.Run(inspectCtx, e.cfg.DockerBinary, []string{"inspect", name}, "")
	cancel()
	if err == nil {
		return nil
	}
	args := []string{
		"create",
		"--name", name,
		"--network", "none",
		"--memory", e.cfg.Memory,
		"--cpus", e.cfg.CPUs,
		"--pids-limit", "128",
		"--read-only",
		"--tmpfs", "/work:rw,exec,nosuid,size=64m",
		"--tmpfs", "/tmp:rw,exec,nosuid,size=64m",
		"-w", "/work",
	}
	for _, env := range spec.Env {
		args = append(args, "-e", env)
	}
	args = append(args, spec.Image, "sleep", "infinity")
	return e.runLifecycle(ctx, args...)
}

func (e *DockerWarmEvaluator) runLifecycle(ctx context.Context, args ...string) error {
	lifecycleCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := e.runner.Run(lifecycleCtx, e.cfg.DockerBinary, args, "")
	return err
}

func (e *DockerWarmEvaluator) containerName(language string) string {
	return strings.ReplaceAll(e.prefix+"-"+language, "_", "-")
}

func (e *DockerWarmEvaluator) finalResult(status string, spec dockerRunnerSpec, containerName string, passedWeight int, totalWeight int, fullResults []map[string]any, publicResults []map[string]any, stdoutText string, stderrText string, message string) (JudgeResult, error) {
	base := &DockerEvaluator{cfg: e.cfg}
	result, err := base.finalResult(status, spec, passedWeight, totalWeight, fullResults, publicResults, stdoutText, stderrText, message)
	if err != nil {
		return JudgeResult{}, err
	}
	result.ResourceUsage["sandbox"] = "docker_warm"
	result.ResourceUsage["container_name"] = containerName
	result.ResourceUsage["lifecycle"] = "create_once_start_exec_stop_tmpfs_reset"
	result.Result["container_reused"] = true
	return result, nil
}

func (e *DockerWarmEvaluator) DebugContainerName(language string) string {
	base := &DockerEvaluator{cfg: e.cfg}
	spec, ok := base.runnerSpec(language)
	if !ok {
		return fmt.Sprintf("%s-unsupported", e.prefix)
	}
	return e.containerName(spec.Language)
}
