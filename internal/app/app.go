package app

import (
	"context"
	"log/slog"
	"net/http"

	"ai-interview-platform/internal/coding"
	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/contextengine"
	"ai-interview-platform/internal/httpapi"
	"ai-interview-platform/internal/provider"
	airuntime "ai-interview-platform/internal/runtime"
	"ai-interview-platform/internal/skill"
	"ai-interview-platform/internal/store"
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
	dbStore, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := dbStore.UpsertProviderConfigs(context.Background(), providerRegistry.List()); err != nil {
		_ = dbStore.Close()
		return nil, err
	}
	if err := dbStore.SyncSkills(context.Background(), skillRegistry.All()); err != nil {
		_ = dbStore.Close()
		return nil, err
	}

	engine := contextengine.New(cfg.TokenBudget, skillRegistry)
	codingStore := coding.NewStore(dbStore.DB())
	runtimeClient := airuntime.NewClient(cfg.AIRuntimeURL)

	router := httpapi.NewRouter(httpapi.Dependencies{
		Config:           cfg,
		Logger:           logger,
		ProviderRegistry: providerRegistry,
		SkillRegistry:    skillRegistry,
		ContextEngine:    engine,
		Store:            dbStore,
		CodingStore:      codingStore,
		RuntimeClient:    runtimeClient,
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
