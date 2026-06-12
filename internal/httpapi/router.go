package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"ai-interview-platform/internal/coding"
	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/contextengine"
	"ai-interview-platform/internal/provider"
	airuntime "ai-interview-platform/internal/runtime"
	"ai-interview-platform/internal/skill"
	"ai-interview-platform/internal/store"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Config           config.Config
	Logger           *slog.Logger
	ProviderRegistry *provider.Registry
	SkillRegistry    *skill.Registry
	ContextEngine    *contextengine.Engine
	Store            *store.Store
	CodingStore      *coding.Store
	RuntimeClient    *airuntime.Client
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
	group.POST("/agent/tasks", api.runAgentTask)
	group.GET("/coding/question-sets", api.listQuestionSets)
	group.GET("/coding/questions", api.listQuestions)
	group.GET("/coding/questions/:question_id", api.getQuestion)

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
	if h.deps.Store != nil {
		if err := h.deps.Store.SyncSkills(c.Request.Context(), h.deps.SkillRegistry.All()); err != nil {
			writeGinError(c, http.StatusInternalServerError, "skill_sync_failed", err.Error())
			return
		}
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

type agentTaskRequest struct {
	TaskType  string `json:"task_type"`
	SkillID   string `json:"skill_id"`
	UserInput string `json:"user_input"`
	DryRun    bool   `json:"dry_run"`
}

func (h apiHandler) runAgentTask(c *gin.Context) {
	var req agentTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	preview, err := h.deps.ContextEngine.Preview(c.Request.Context(), contextengine.PreviewRequest{
		TaskType: req.TaskType,
		SkillID:  req.SkillID,
	})
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "context_preview_failed", err.Error())
		return
	}
	runtimeResp, err := h.deps.RuntimeClient.RunTask(c.Request.Context(), airuntime.TaskRequest{
		TaskType:     req.TaskType,
		ContextItems: preview.Items,
		UserInput:    req.UserInput,
		DryRun:       req.DryRun,
	})
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_task_failed", err.Error())
		return
	}
	traceID := store.NewID("trace")
	if h.deps.Store != nil {
		_ = h.deps.Store.InsertAgentTrace(c.Request.Context(), store.TraceRecord{
			TraceID:      traceID,
			TaskType:     req.TaskType,
			SkillID:      req.SkillID,
			Input:        req,
			ContextItems: preview.Items,
			Output:       runtimeResp,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version":   "agent.task.v1",
		"trace_id":         traceID,
		"context_preview":  preview,
		"runtime_response": runtimeResp,
	})
}

func (h apiHandler) listQuestionSets(c *gin.Context) {
	items, err := h.deps.CodingStore.ListSets(c.Request.Context())
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "coding_sets_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "coding.question_sets.v1", "items": items})
}

func (h apiHandler) listQuestions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := h.deps.CodingStore.ListQuestions(c.Request.Context(), c.Query("question_type"), limit)
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "coding_questions_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "coding.questions.v1", "items": items})
}

func (h apiHandler) getQuestion(c *gin.Context) {
	item, ok, err := h.deps.CodingStore.GetQuestion(c.Request.Context(), c.Param("question_id"))
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "coding_question_failed", err.Error())
		return
	}
	if !ok {
		writeGinError(c, http.StatusNotFound, "coding_question_not_found", "question_id is not registered")
		return
	}
	c.JSON(http.StatusOK, item)
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
