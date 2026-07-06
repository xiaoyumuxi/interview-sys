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

type NativeEvaluatorConfig struct {
	GoBinary         string
	JavaBinary       string
	JavaCompiler     string
	PythonBinary     string
	JavaScriptBinary string
	TypeScriptBinary string
	CppCompiler      string
	Timeout          time.Duration
	WorkDir          string
	MaxOutputBytes   int
}

type NativeEvaluator struct {
	cfg    NativeEvaluatorConfig
	runner nativeCommandRunner
}

type nativeCommandRunner interface {
	Run(ctx context.Context, workDir string, command string, stdin string) (commandResult, error)
}

type nativeRunnerSpec struct {
	Language string
	FileName string
	Command  string
}

func NewNativeEvaluator(cfg NativeEvaluatorConfig) *NativeEvaluator {
	if cfg.GoBinary == "" {
		cfg.GoBinary = "go"
	}
	if cfg.JavaBinary == "" {
		cfg.JavaBinary = "java"
	}
	if cfg.JavaCompiler == "" {
		cfg.JavaCompiler = "javac"
	}
	if cfg.PythonBinary == "" {
		cfg.PythonBinary = "python3"
	}
	if cfg.JavaScriptBinary == "" {
		cfg.JavaScriptBinary = "node"
	}
	if cfg.TypeScriptBinary == "" {
		cfg.TypeScriptBinary = "deno"
	}
	if cfg.CppCompiler == "" {
		cfg.CppCompiler = "g++"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 64 * 1024
	}
	return &NativeEvaluator{cfg: cfg, runner: osNativeCommandRunner{maxOutputBytes: cfg.MaxOutputBytes}}
}

func (e *NativeEvaluator) Evaluate(ctx context.Context, submission Submission, question Question, cases []TestCase) (JudgeResult, error) {
	_ = question
	spec, ok := e.runnerSpec(submission.Language)
	if !ok {
		return JudgeResult{
			Status: StatusSystemError,
			Result: map[string]any{
				"error_code": "unsupported_language_for_native_judge",
				"message":    "native trusted judge does not support this language",
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
	dir, err := os.MkdirTemp(e.cfg.WorkDir, "coding-native-judge-*")
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
		result, err := e.runner.Run(runCtx, dir, spec.Command, testCase.InputText)
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

func (e *NativeEvaluator) runnerSpec(language string) (nativeRunnerSpec, bool) {
	switch normalizeLanguage(language) {
	case "go":
		return nativeRunnerSpec{Language: "go", FileName: "Main.go", Command: e.cfg.GoBinary + " run Main.go"}, true
	case "java":
		return nativeRunnerSpec{Language: "java", FileName: "Main.java", Command: fmt.Sprintf("mkdir -p classes && %s Main.java -d classes && %s -cp classes Main", e.cfg.JavaCompiler, e.cfg.JavaBinary)}, true
	case "python", "python3":
		return nativeRunnerSpec{Language: "python", FileName: "Main.py", Command: e.cfg.PythonBinary + " Main.py"}, true
	case "javascript":
		return nativeRunnerSpec{Language: "javascript", FileName: "Main.js", Command: e.cfg.JavaScriptBinary + " Main.js"}, true
	case "typescript":
		return nativeRunnerSpec{Language: "typescript", FileName: "Main.ts", Command: e.cfg.TypeScriptBinary + " run --no-prompt --no-net Main.ts"}, true
	case "cpp", "c++":
		return nativeRunnerSpec{Language: "cpp", FileName: "Main.cpp", Command: e.cfg.CppCompiler + " -std=c++20 -O2 -pipe Main.cpp -o main && ./main"}, true
	default:
		return nativeRunnerSpec{}, false
	}
}

func (e *NativeEvaluator) finalResult(status string, spec nativeRunnerSpec, passedWeight int, totalWeight int, fullResults []map[string]any, publicResults []map[string]any, stdoutText string, stderrText string, message string) (JudgeResult, error) {
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
			"sandbox":  "native_trusted",
			"language": spec.Language,
			"timeout":  e.cfg.Timeout.String(),
		},
	}, nil
}

type osNativeCommandRunner struct {
	maxOutputBytes int
}

func (r osNativeCommandRunner) Run(ctx context.Context, workDir string, command string, stdin string) (commandResult, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
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
