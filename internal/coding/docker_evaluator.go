package coding

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DockerEvaluatorConfig struct {
	DockerBinary   string
	GoImage        string
	Timeout        time.Duration
	Memory         string
	CPUs           string
	WorkDir        string
	MaxOutputBytes int
}

type DockerEvaluator struct {
	cfg    DockerEvaluatorConfig
	runner dockerCommandRunner
}

type dockerCommandRunner interface {
	Run(ctx context.Context, name string, args []string, stdin string) (commandResult, error)
}

type commandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

func NewDockerEvaluator(cfg DockerEvaluatorConfig) *DockerEvaluator {
	if cfg.DockerBinary == "" {
		cfg.DockerBinary = "docker"
	}
	if cfg.GoImage == "" {
		cfg.GoImage = "golang:1.26-alpine"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Memory == "" {
		cfg.Memory = "128m"
	}
	if cfg.CPUs == "" {
		cfg.CPUs = "0.5"
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 64 * 1024
	}
	return &DockerEvaluator{cfg: cfg, runner: osCommandRunner{maxOutputBytes: cfg.MaxOutputBytes}}
}

func (e *DockerEvaluator) Evaluate(ctx context.Context, submission Submission, question Question, cases []TestCase) (JudgeResult, error) {
	_ = question
	if normalizeLanguage(submission.Language) != "go" {
		return JudgeResult{
			Status: StatusSystemError,
			Result: map[string]any{
				"error_code": "unsupported_language_for_docker_judge",
				"message":    "docker judge currently supports go submissions only",
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
	dir, err := os.MkdirTemp(e.cfg.WorkDir, "coding-judge-*")
	if err != nil {
		return JudgeResult{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, "Main.go"), []byte(submission.SourceCode), 0o600); err != nil {
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
		result, err := e.runner.Run(runCtx, e.cfg.DockerBinary, e.dockerArgs(dir), testCase.InputText)
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
			return e.finalResult(StatusTimeLimitExceeded, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), "time limit exceeded")
		}
		if result.ExitCode != 0 {
			caseResult["passed"] = false
			fullResults = append(fullResults, caseResult)
			publicResults = append(publicResults, publicCaseResult(index, caseResult))
			status := StatusRuntimeError
			if looksLikeGoCompileError(result.Stderr) {
				status = StatusCompileError
			}
			return e.finalResult(status, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), "program exited with non-zero status")
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
	return e.finalResult(status, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), message)
}

func (e *DockerEvaluator) dockerArgs(dir string) []string {
	return []string{
		"run",
		"--rm",
		"--network", "none",
		"--memory", e.cfg.Memory,
		"--cpus", e.cfg.CPUs,
		"--pids-limit", "128",
		"--read-only",
		"--tmpfs", "/tmp:rw,exec,nosuid,size=64m",
		"-e", "GOCACHE=/tmp/gocache",
		"-v", dir + ":/work:ro",
		"-w", "/work",
		e.cfg.GoImage,
		"go", "run", "./Main.go",
	}
}

func (e *DockerEvaluator) finalResult(status string, passedWeight int, totalWeight int, fullResults []map[string]any, publicResults []map[string]any, stdoutText string, stderrText string, message string) (JudgeResult, error) {
	score := 0.0
	if totalWeight > 0 {
		score = float64(passedWeight) / float64(totalWeight) * 100
	}
	return JudgeResult{
		Status: status,
		Score:  score,
		Result: map[string]any{
			"message":       message,
			"passed_weight": passedWeight,
			"total_weight":  totalWeight,
			"test_results":  publicResults,
		},
		TestResults: fullResults,
		StdoutText:  limitText(stdoutText, e.cfg.MaxOutputBytes),
		StderrText:  limitText(stderrText, e.cfg.MaxOutputBytes),
		ResourceUsage: map[string]any{
			"sandbox": "docker",
			"image":   e.cfg.GoImage,
			"timeout": e.cfg.Timeout.String(),
			"memory":  e.cfg.Memory,
			"cpus":    e.cfg.CPUs,
		},
	}, nil
}

type osCommandRunner struct {
	maxOutputBytes int
}

func (r osCommandRunner) Run(ctx context.Context, name string, args []string, stdin string) (commandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := commandResult{
		Stdout: limitText(stdout.String(), r.maxOutputBytes),
		Stderr: limitText(stderr.String(), r.maxOutputBytes),
	}
	if ctx.Err() != nil {
		result.TimedOut = true
		return result, nil
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func publicCaseResult(index int, full map[string]any) map[string]any {
	caseType, _ := full["case_type"].(string)
	out := map[string]any{
		"index":        index,
		"case_type":    caseType,
		"passed":       full["passed"],
		"exit_code":    full["exit_code"],
		"timed_out":    full["timed_out"],
		"stdout":       full["stdout"],
		"stderr":       full["stderr"],
		"test_case_id": full["test_case_id"],
	}
	if caseType == "sample" {
		out["input_text"] = full["input_text"]
		out["expected_output"] = full["expected_output"]
	} else {
		out["hidden"] = true
	}
	return out
}

func normalizeOutput(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func looksLikeGoCompileError(stderr string) bool {
	text := strings.ToLower(stderr)
	return strings.Contains(text, "# command-line-arguments") ||
		strings.Contains(text, "syntax error") ||
		strings.Contains(text, "undefined:")
}

func limitText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + fmt.Sprintf("\n... output truncated to %d bytes", limit)
}
