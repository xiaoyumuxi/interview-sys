package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"ai-interview-platform/internal/auth"
	"ai-interview-platform/internal/coding"
	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/contextengine"
	"ai-interview-platform/internal/evalharness"
	"ai-interview-platform/internal/httpapi"
	"ai-interview-platform/internal/interview"
	"ai-interview-platform/internal/provider"
	airuntime "ai-interview-platform/internal/runtime"
	"ai-interview-platform/internal/singleflight"
	"ai-interview-platform/internal/skill"
	"ai-interview-platform/internal/store"
	"ai-interview-platform/internal/workqueue"

	"github.com/redis/go-redis/v9"
)

type App struct {
	cfg    config.Config
	logger *slog.Logger
	router http.Handler
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	skillRegistry := skill.NewRegistry(cfg.SkillsDir)
	if err := skillRegistry.Load(); err != nil {
		return nil, err
	}

	providerRegistry := provider.NewRegistry()
	dbStore, err := store.Open(cfg.DatabaseURL, cfg.ProviderKeyEncryptionSecret)
	if err != nil {
		return nil, err
	}
	if err := dbStore.SeedProviderConfigs(context.Background(), providerRegistry.List()); err != nil {
		_ = dbStore.Close()
		return nil, err
	}
	if err := dbStore.SyncSkills(context.Background(), skillRegistry.All()); err != nil {
		_ = dbStore.Close()
		return nil, err
	}
	authService := auth.NewService(cfg, dbStore)
	if _, err := authService.BootstrapRoot(context.Background(), "root", cfg.RootDisplayName, cfg.RootEmail, cfg.RootPassword); err != nil {
		_ = dbStore.Close()
		return nil, err
	}

	codingStore := coding.NewStore(dbStore.DB())
	runtimeClient := airuntime.NewClient(cfg.AIRuntimeURL)
	engine := contextengine.New(cfg.TokenBudget, skillRegistry)
	engine.SetMemorySource(runtimeClient)
	evaluationService := evalharness.NewService(evalharness.NewStore(dbStore.DB()), dbStore, engine, runtimeClient)
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("redis unavailable; single-flight and stream queue will degrade", "addr", cfg.RedisAddr, "error", err)
	}
	stream := workqueue.NewStreamWithDeadLetter(redisClient, logger, cfg.InterviewEventsStream, cfg.InterviewDeadLetterStream)
	flights := singleflight.NewRedisFlight(redisClient, 65*time.Second, 10*time.Minute)
	interviewService := interview.NewService(dbStore.DB(), dbStore, engine, runtimeClient, flights, stream)
	engine.SetRecentHistorySource(interviewService)
	if cfg.EnableEmbeddedWorker {
		logger.Warn("embedded interview worker is enabled; prefer running cmd/worker in normal development and deployment")
		interviewService.StartWorker(context.Background(), interview.DefaultWorkerOptions("api"))
	}

	router := httpapi.NewRouter(httpapi.Dependencies{
		Config:           cfg,
		Logger:           logger,
		ProviderRegistry: providerRegistry,
		SkillRegistry:    skillRegistry,
		ContextEngine:    engine,
		Store:            dbStore,
		CodingStore:      codingStore,
		AuthService:      authService,
		RuntimeClient:    runtimeClient,
		InterviewService: interviewService,
		Evaluation:       evaluationService,
	})

	return &App{
		cfg:    cfg,
		logger: logger,
		router: router,
	}, nil
}

func (a *App) Routes() http.Handler {
	return a.router
}
