package app

import (
	"log/slog"
	"net/http"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/contextengine"
	"ai-interview-platform/internal/httpapi"
	"ai-interview-platform/internal/provider"
	"ai-interview-platform/internal/skill"
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
	engine := contextengine.New(cfg.TokenBudget, skillRegistry)

	router := httpapi.NewRouter(httpapi.Dependencies{
		Config:           cfg,
		Logger:           logger,
		ProviderRegistry: providerRegistry,
		SkillRegistry:    skillRegistry,
		ContextEngine:    engine,
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
