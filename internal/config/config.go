package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv                      string
	HTTPAddr                    string
	LogLevel                    slog.Level
	DatabaseURL                 string
	RedisAddr                   string
	InterviewEventsStream       string
	InterviewDeadLetterStream   string
	EnableEmbeddedWorker        bool
	CodingJudgeEnabled          bool
	CodingJudgeBatchSize        int
	CodingJudgeMode             string
	CodingJudgeDockerBinary     string
	CodingJudgeContainerPrefix  string
	CodingJudgeGoImage          string
	CodingJudgeJavaImage        string
	CodingJudgePythonImage      string
	CodingJudgeJavaScriptImage  string
	CodingJudgeTypeScriptImage  string
	CodingJudgeCppImage         string
	CodingJudgeNativeGo         string
	CodingJudgeNativeJava       string
	CodingJudgeNativeJavac      string
	CodingJudgeNativePython     string
	CodingJudgeNativeNode       string
	CodingJudgeNativeDeno       string
	CodingJudgeNativeGpp        string
	CodingJudgeTimeoutSeconds   int
	CodingJudgeMemory           string
	CodingJudgeCPUs             string
	MinIOEndpoint               string
	SkillsDir                   string
	AIRuntimeURL                string
	TokenBudget                 int
	ProviderKeyEncryptionSecret string
	AuthDisabled                bool
	JWTAccessSecret             string
	JWTRefreshSecret            string
	AccessTokenTTLMinutes       int
	RefreshTokenTTLDays         int
	RootEmail                   string
	RootPassword                string
	RootDisplayName             string
}

func Load() Config {
	return Config{
		AppEnv:                      env("APP_ENV", "local"),
		HTTPAddr:                    env("HTTP_ADDR", ":8080"),
		LogLevel:                    parseLogLevel(env("LOG_LEVEL", "info")),
		DatabaseURL:                 env("DATABASE_URL", "postgres://ai_interview:ai_interview@localhost:5432/ai_interview?sslmode=disable"),
		RedisAddr:                   env("REDIS_ADDR", "localhost:6379"),
		InterviewEventsStream:       env("INTERVIEW_EVENTS_STREAM", "interview:events"),
		InterviewDeadLetterStream:   env("INTERVIEW_DEAD_LETTER_STREAM", "interview:events:dead"),
		EnableEmbeddedWorker:        envBool("ENABLE_EMBEDDED_WORKER", false),
		CodingJudgeEnabled:          envBool("CODING_JUDGE_ENABLED", false),
		CodingJudgeBatchSize:        envInt("CODING_JUDGE_BATCH_SIZE", 4),
		CodingJudgeMode:             env("CODING_JUDGE_MODE", "disabled"),
		CodingJudgeDockerBinary:     env("CODING_JUDGE_DOCKER_BINARY", "docker"),
		CodingJudgeContainerPrefix:  env("CODING_JUDGE_CONTAINER_PREFIX", "ai-interview-judge"),
		CodingJudgeGoImage:          env("CODING_JUDGE_GO_IMAGE", "golang:1.26-alpine"),
		CodingJudgeJavaImage:        env("CODING_JUDGE_JAVA_IMAGE", "eclipse-temurin:21-jdk-alpine"),
		CodingJudgePythonImage:      env("CODING_JUDGE_PYTHON_IMAGE", "python:3.13-alpine"),
		CodingJudgeJavaScriptImage:  env("CODING_JUDGE_JAVASCRIPT_IMAGE", "node:22-alpine"),
		CodingJudgeTypeScriptImage:  env("CODING_JUDGE_TYPESCRIPT_IMAGE", "denoland/deno:alpine-2.1.4"),
		CodingJudgeCppImage:         env("CODING_JUDGE_CPP_IMAGE", "gcc:14-alpine"),
		CodingJudgeNativeGo:         env("CODING_JUDGE_NATIVE_GO", "go"),
		CodingJudgeNativeJava:       env("CODING_JUDGE_NATIVE_JAVA", "java"),
		CodingJudgeNativeJavac:      env("CODING_JUDGE_NATIVE_JAVAC", "javac"),
		CodingJudgeNativePython:     env("CODING_JUDGE_NATIVE_PYTHON", "python3"),
		CodingJudgeNativeNode:       env("CODING_JUDGE_NATIVE_NODE", "node"),
		CodingJudgeNativeDeno:       env("CODING_JUDGE_NATIVE_DENO", "deno"),
		CodingJudgeNativeGpp:        env("CODING_JUDGE_NATIVE_GPP", "g++"),
		CodingJudgeTimeoutSeconds:   envInt("CODING_JUDGE_TIMEOUT_SECONDS", 5),
		CodingJudgeMemory:           env("CODING_JUDGE_MEMORY", "128m"),
		CodingJudgeCPUs:             env("CODING_JUDGE_CPUS", "0.5"),
		MinIOEndpoint:               env("MINIO_ENDPOINT", "localhost:9000"),
		SkillsDir:                   env("SKILLS_DIR", "skills"),
		AIRuntimeURL:                env("AI_RUNTIME_URL", "http://localhost:8090"),
		TokenBudget:                 envInt("CONTEXT_TOKEN_BUDGET", 12000),
		ProviderKeyEncryptionSecret: env("PROVIDER_KEY_ENCRYPTION_SECRET", ""),
		AuthDisabled:                envBool("AUTH_DISABLED", false),
		JWTAccessSecret:             env("JWT_ACCESS_SECRET", "local-dev-access-secret-change-me"),
		JWTRefreshSecret:            env("JWT_REFRESH_SECRET", "local-dev-refresh-secret-change-me"),
		AccessTokenTTLMinutes:       envInt("ACCESS_TOKEN_TTL_MINUTES", 15),
		RefreshTokenTTLDays:         envInt("REFRESH_TOKEN_TTL_DAYS", 30),
		RootEmail:                   env("ROOT_EMAIL", "root@example.local"),
		RootPassword:                env("ROOT_PASSWORD", "RootChangeMe123!"),
		RootDisplayName:             env("ROOT_DISPLAY_NAME", "Root"),
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
