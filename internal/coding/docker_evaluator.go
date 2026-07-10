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
	DockerBinary    string
	GoImage         string
	JavaImage       string
	PythonImage     string
	JavaScriptImage string
	TypeScriptImage string
	CppImage        string
	Timeout         time.Duration
	Memory          string
	CPUs            string
	WorkDir         string
	MaxOutputBytes  int
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

type dockerRunnerSpec struct {
	Language string
	Image    string
	FileName string
	Env      []string
	Command  string
}

func NewDockerEvaluator(cfg DockerEvaluatorConfig) *DockerEvaluator {
	if cfg.DockerBinary == "" {
		cfg.DockerBinary = "docker"
	}
	if cfg.GoImage == "" {
		cfg.GoImage = "golang:1.26-alpine"
	}
	if cfg.JavaImage == "" {
		cfg.JavaImage = "eclipse-temurin:21-jdk-alpine"
	}
	if cfg.PythonImage == "" {
		cfg.PythonImage = "python:3.13-alpine"
	}
	if cfg.JavaScriptImage == "" {
		cfg.JavaScriptImage = "node:22-alpine"
	}
	if cfg.TypeScriptImage == "" {
		cfg.TypeScriptImage = "denoland/deno:alpine-2.1.4"
	}
	if cfg.CppImage == "" {
		cfg.CppImage = "gcc:14-alpine"
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
	spec, ok := e.runnerSpec(submission.Language)
	if !ok {
		return JudgeResult{
			Status: StatusSystemError,
			Result: map[string]any{
				"error_code": "unsupported_language_for_docker_judge",
				"message":    "docker judge does not support this language",
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
	if err := os.WriteFile(filepath.Join(dir, spec.FileName), []byte(submission.SourceCode), 0o600); err != nil {
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
		result, err := e.runner.Run(runCtx, e.cfg.DockerBinary, e.dockerArgs(dir, spec), testCase.InputText)
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
			return e.finalResult(StatusTimeLimitExceeded, spec, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), "time limit exceeded")
		}
		if result.ExitCode != 0 {
			caseResult["passed"] = false
			fullResults = append(fullResults, caseResult)
			publicResults = append(publicResults, publicCaseResult(index, caseResult))
			status := StatusRuntimeError
			if looksLikeCompileError(spec.Language, result.Stderr) {
				status = StatusCompileError
			}
			return e.finalResult(status, spec, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), "program exited with non-zero status")
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
	return e.finalResult(status, spec, passedWeight, totalWeight, fullResults, publicResults, stdoutCombined.String(), stderrCombined.String(), message)
}

func (e *DockerEvaluator) runnerSpec(language string) (dockerRunnerSpec, bool) {
	switch normalizeLanguage(language) {
	case "go":
		return dockerRunnerSpec{
			Language: "go",
			Image:    e.cfg.GoImage,
			FileName: "Main.go",
			Env:      []string{"GOCACHE=/tmp/gocache"},
			Command:  "go run /work/Main.go",
		}, true
	case "java":
		return dockerRunnerSpec{
			Language: "java",
			Image:    e.cfg.JavaImage,
			FileName: "Main.java",
			Command:  "mkdir -p /tmp/classes && javac /work/Main.java -d /tmp/classes && java -cp /tmp/classes Main",
		}, true
	case "python", "python3":
		return dockerRunnerSpec{
			Language: "python",
			Image:    e.cfg.PythonImage,
			FileName: "Main.py",
			Command:  "python /work/Main.py",
		}, true
	case "javascript":
		return dockerRunnerSpec{
			Language: "javascript",
			Image:    e.cfg.JavaScriptImage,
			FileName: "Main.js",
			Command:  "node /work/Main.js",
		}, true
	case "typescript":
		return dockerRunnerSpec{
			Language: "typescript",
			Image:    e.cfg.TypeScriptImage,
			FileName: "Main.ts",
			Env:      []string{"DENO_DIR=/tmp/deno"},
			Command:  "deno run --no-prompt --no-net /work/Main.ts",
		}, true
	case "cpp", "c++":
		return dockerRunnerSpec{
			Language: "cpp",
			Image:    e.cfg.CppImage,
			FileName: "Main.cpp",
			Command:  "g++ -std=c++20 -O2 -pipe /work/Main.cpp -o /tmp/main && /tmp/main",
		}, true
	default:
		return dockerRunnerSpec{}, false
	}
}

func (e *DockerEvaluator) dockerArgs(dir string, spec dockerRunnerSpec) []string {
	args := []string{
		"run",
		"--rm",
		"--network", "none",
		"--memory", e.cfg.Memory,
		"--cpus", e.cfg.CPUs,
		"--pids-limit", "128",
		"--read-only",
		"--tmpfs", "/tmp:rw,exec,nosuid,size=64m",
		"-v", dir + ":/work:ro",
		"-w", "/work",
	}
	for _, env := range spec.Env {
		args = append(args, "-e", env)
	}
	args = append(args, spec.Image, "sh", "-c", spec.Command)
	return args
}

func (e *DockerEvaluator) finalResult(status string, spec dockerRunnerSpec, passedWeight int, totalWeight int, fullResults []map[string]any, publicResults []map[string]any, stdoutText string, stderrText string, message string) (JudgeResult, error) {
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
			"sandbox":  "docker",
			"language": spec.Language,
			"image":    spec.Image,
			"timeout":  e.cfg.Timeout.String(),
			"memory":   e.cfg.Memory,
			"cpus":     e.cfg.CPUs,
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

func looksLikeCompileError(language string, stderr string) bool {
	text := strings.ToLower(stderr)
	switch language {
	case "go":
		return strings.Contains(text, "# command-line-arguments") ||
			strings.Contains(text, "syntax error") ||
			strings.Contains(text, "undefined:")
	case "java", "cpp":
		return strings.Contains(text, "error:")
	case "python", "javascript":
		return strings.Contains(text, "syntaxerror")
	case "typescript":
		return strings.Contains(text, "syntaxerror") || strings.Contains(text, "ts")
	default:
		return false
	}
}

func limitText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + fmt.Sprintf("\n... output truncated to %d bytes", limit)
}
