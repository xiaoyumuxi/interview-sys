package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/contextengine"
	"ai-interview-platform/internal/interview"
	"ai-interview-platform/internal/provider"
	airuntime "ai-interview-platform/internal/runtime"
	"ai-interview-platform/internal/singleflight"
	"ai-interview-platform/internal/skill"
	"ai-interview-platform/internal/store"
	"ai-interview-platform/internal/workqueue"

	"github.com/redis/go-redis/v9"
)

func main() {
	loadDotEnv(".env")
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	service, closeFn, err := newInterviewService(ctx, cfg, logger)
	if err != nil {
		logger.Error("initialize worker", "error", err)
		os.Exit(1)
	}
	defer closeFn()

	opts := interview.DefaultWorkerOptions("worker")
	logger.Info("interview worker starting",
		"group", opts.Group,
		"consumer", opts.Consumer,
		"stream", cfg.InterviewEventsStream,
		"dead_letter_stream", cfg.InterviewDeadLetterStream,
	)
	service.StartWorker(ctx, opts)

	<-ctx.Done()
	logger.Info("interview worker stopped")
}

func newInterviewService(ctx context.Context, cfg config.Config, logger *slog.Logger) (*interview.Service, func(), error) {
	skillRegistry := skill.NewRegistry(cfg.SkillsDir)
	if err := skillRegistry.Load(); err != nil {
		return nil, nil, err
	}

	providerRegistry := provider.NewRegistry()
	dbStore, err := store.Open(cfg.DatabaseURL, cfg.ProviderKeyEncryptionSecret)
	if err != nil {
		return nil, nil, err
	}
	closeFn := func() { _ = dbStore.Close() }

	if err := dbStore.SeedProviderConfigs(ctx, providerRegistry.List()); err != nil {
		closeFn()
		return nil, nil, err
	}
	if err := dbStore.SyncSkills(ctx, skillRegistry.All()); err != nil {
		closeFn()
		return nil, nil, err
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		closeFn()
		return nil, nil, err
	}
	closeFn = func() {
		_ = redisClient.Close()
		_ = dbStore.Close()
	}

	runtimeClient := airuntime.NewClient(cfg.AIRuntimeURL)
	engine := contextengine.New(cfg.TokenBudget, skillRegistry)
	engine.SetMemorySource(runtimeClient)
	stream := workqueue.NewStreamWithDeadLetter(redisClient, logger, cfg.InterviewEventsStream, cfg.InterviewDeadLetterStream)
	flights := singleflight.NewRedisFlight(redisClient, 65*time.Second, 10*time.Minute)
	return interview.NewService(dbStore.DB(), dbStore, engine, runtimeClient, flights, stream), closeFn, nil
}

func loadDotEnv(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, strings.Trim(strings.TrimSpace(value), `"'`))
	}
}
