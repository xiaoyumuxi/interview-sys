package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/contextengine"
	"ai-interview-platform/internal/provider"
	"ai-interview-platform/internal/skill"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Config           config.Config
	Logger           *slog.Logger
	ProviderRegistry *provider.Registry
	SkillRegistry    *skill.Registry
	ContextEngine    *contextengine.Engine
}

func NewRouter(deps Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(ginRecovery())
	router.Use(ginRequestLogger(deps.Logger))

	api := apiHandler{deps: deps}
	router.GET("/healthz", api.health)

	group := router.Group("/api")
	group.GET("/providers", api.listProviders)
	group.POST("/providers/test", api.testProvider)
	group.GET("/skills", api.listSkills)
	group.POST("/skills", api.createSkill)
	group.POST("/skills/reload", api.reloadSkills)
	group.GET("/skills/:skill_id", api.getSkill)
	group.POST("/context/preview", api.contextPreview)

	return router
}

type apiHandler struct {
	deps Dependencies
}

func (h apiHandler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"schema_version": "health.v1",
		"app_env":        h.deps.Config.AppEnv,
		"time":           time.Now().Format(time.RFC3339),
	})
}

func (h apiHandler) listProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"schema_version": "provider.list.v1",
		"items":          h.deps.ProviderRegistry.List(),
	})
}

func (h apiHandler) testProvider(c *gin.Context) {
	var req provider.TestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	c.JSON(http.StatusOK, h.deps.ProviderRegistry.Test(c.Request.Context(), req))
}

func (h apiHandler) listSkills(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"schema_version": "skill.list.v1",
		"items":          h.deps.SkillRegistry.List(),
	})
}

func (h apiHandler) createSkill(c *gin.Context) {
	var req skill.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	item, err := h.deps.SkillRegistry.Create(req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "skill_create_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"schema_version": "skill.create.v1",
		"item":           item,
	})
}

func (h apiHandler) reloadSkills(c *gin.Context) {
	if err := h.deps.SkillRegistry.Reload(); err != nil {
		writeGinError(c, http.StatusInternalServerError, "skill_reload_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version": "skill.reload.v1",
		"items":          h.deps.SkillRegistry.List(),
	})
}

func (h apiHandler) getSkill(c *gin.Context) {
	item, ok := h.deps.SkillRegistry.Get(c.Param("skill_id"))
	if !ok {
		writeGinError(c, http.StatusNotFound, "skill_not_found", "skill_id is not registered")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h apiHandler) contextPreview(c *gin.Context) {
	var req contextengine.PreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	resp, err := h.deps.ContextEngine.Preview(c.Request.Context(), req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "context_preview_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, resp)
}

func writeGinError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}

func ginRequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

func ginRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		writeGinError(c, http.StatusInternalServerError, "internal_error", "request failed")
	})
}
