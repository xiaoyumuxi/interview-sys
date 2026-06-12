package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv        string
	HTTPAddr      string
	LogLevel      slog.Level
	DatabaseURL   string
	RedisAddr     string
	MinIOEndpoint string
	SkillsDir     string
	AIRuntimeURL  string
	TokenBudget   int
}

func Load() Config {
	return Config{
		AppEnv:        env("APP_ENV", "local"),
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		LogLevel:      parseLogLevel(env("LOG_LEVEL", "info")),
		DatabaseURL:   env("DATABASE_URL", "postgres://ai_interview:ai_interview@localhost:5432/ai_interview?sslmode=disable"),
		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		MinIOEndpoint: env("MINIO_ENDPOINT", "localhost:9000"),
		SkillsDir:     env("SKILLS_DIR", "skills"),
		AIRuntimeURL:  env("AI_RUNTIME_URL", "http://localhost:8090"),
		TokenBudget:   envInt("CONTEXT_TOKEN_BUDGET", 12000),
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
