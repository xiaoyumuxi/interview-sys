package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"ai-interview-platform/internal/auth"
	"ai-interview-platform/internal/coding"
	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/contextengine"
	"ai-interview-platform/internal/interview"
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
	AuthService      *auth.Service
	RuntimeClient    *airuntime.Client
	InterviewService *interview.Service
}

func NewRouter(deps Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(ginRecovery())
	router.Use(ginRequestLogger(deps.Logger))

	api := apiHandler{deps: deps}
	router.GET("/healthz", api.health)

	authGroup := router.Group("/api/auth")
	authGroup.POST("/register", api.register)
	authGroup.POST("/login", api.login)
	authGroup.POST("/refresh", api.refreshToken)
	authGroup.POST("/logout", api.logout)
	authGroup.GET("/me", api.requireAuth(), api.me)

	group := router.Group("/api")
	group.Use(api.requireAuth())
	group.GET("/providers", api.requireRoot(), api.listProviders)
	group.POST("/providers", api.requireRoot(), api.createProvider)
	group.GET("/providers/:provider_id", api.requireRoot(), api.getProvider)
	group.PUT("/providers/:provider_id", api.requireRoot(), api.updateProvider)
	group.DELETE("/providers/:provider_id", api.requireRoot(), api.deleteProvider)
	group.POST("/providers/test", api.requireRoot(), api.testProvider)
	group.GET("/provider-routes", api.requireRoot(), api.listProviderRoutes)
	group.PUT("/provider-routes/:task_type", api.requireRoot(), api.updateProviderRoute)
	group.GET("/skills", api.listSkills)
	group.POST("/skills", api.requireRoot(), api.createSkill)
	group.POST("/skills/reload", api.requireRoot(), api.reloadSkills)
	group.GET("/skills/:skill_id", api.getSkill)
	group.POST("/context/preview", api.contextPreview)
	group.POST("/agent/tasks", api.runAgentTask)
	group.POST("/interview-sessions", api.createInterviewSession)
	group.GET("/interview-sessions/:session_id", api.getInterviewSession)
	group.POST("/interview-sessions/:session_id/answers", api.submitInterviewAnswer)
	group.POST("/interview-sessions/:session_id/finalize", api.finalizeInterviewSession)
	group.GET("/interview-sessions/:session_id/trace", api.getInterviewTrace)
	group.GET("/coding/question-sets", api.listQuestionSets)
	group.GET("/coding/questions", api.listQuestions)
	group.GET("/coding/questions/:question_id", api.getQuestion)
	group.GET("/ops/dead-letters/summary", api.requireRoot(), api.deadLetterSummary)
	group.GET("/ops/dead-letters", api.requireRoot(), api.listDeadLetters)
	group.GET("/ops/dead-letters/:dead_letter_id", api.requireRoot(), api.getDeadLetter)
	group.GET("/ops/workers/summary", api.requireRoot(), api.workerSummary)

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
	items, err := h.deps.Store.ListProviders(c.Request.Context())
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "provider_list_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version": "provider.list.v1",
		"items":          items,
	})
}

func (h apiHandler) createProvider(c *gin.Context) {
	var req provider.SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	item, err := h.deps.Store.SaveProvider(c.Request.Context(), req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "provider_save_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"schema_version": "provider.item.v1", "item": item})
}

func (h apiHandler) getProvider(c *gin.Context) {
	item, ok, err := h.deps.Store.GetProvider(c.Request.Context(), c.Param("provider_id"))
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "provider_get_failed", err.Error())
		return
	}
	if !ok {
		writeGinError(c, http.StatusNotFound, "provider_not_found", "provider_id is not registered")
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "provider.item.v1", "item": item})
}

func (h apiHandler) updateProvider(c *gin.Context) {
	var req provider.SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	req.ProviderID = c.Param("provider_id")
	item, err := h.deps.Store.SaveProvider(c.Request.Context(), req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "provider_save_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "provider.item.v1", "item": item})
}

func (h apiHandler) deleteProvider(c *gin.Context) {
	if err := h.deps.Store.DeleteProvider(c.Request.Context(), c.Param("provider_id")); err != nil {
		writeGinError(c, http.StatusBadRequest, "provider_delete_failed", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (h apiHandler) testProvider(c *gin.Context) {
	var req provider.TestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	cfg, apiKey, ok, err := h.deps.Store.ProviderTestConfig(c.Request.Context(), req.ProviderID)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "provider_test_failed", err.Error())
		return
	}
	if !ok {
		c.JSON(http.StatusOK, provider.TestResponse{SchemaVersion: "provider.test.v1", ProviderID: req.ProviderID, OK: false, Message: "provider not found"})
		return
	}
	if !cfg.Enabled {
		c.JSON(http.StatusOK, provider.TestResponse{SchemaVersion: "provider.test.v1", ProviderID: cfg.ProviderID, OK: false, Message: "provider is disabled"})
		return
	}
	if cfg.ProviderType == provider.TypeEmbedding {
		c.JSON(http.StatusOK, provider.TestResponse{SchemaVersion: "provider.test.v1", ProviderID: cfg.ProviderID, OK: apiKey != "", Message: "embedding provider config is present"})
		return
	}
	if err := provider.TestChat(c.Request.Context(), cfg, apiKey, req.Prompt); err != nil {
		c.JSON(http.StatusOK, provider.TestResponse{SchemaVersion: "provider.test.v1", ProviderID: cfg.ProviderID, OK: false, Message: err.Error(), Endpoint: provider.ChatEndpoint(cfg), Model: cfg.ChatModel})
		return
	}
	c.JSON(http.StatusOK, provider.TestResponse{SchemaVersion: "provider.test.v1", ProviderID: cfg.ProviderID, OK: true, Message: "chat completion request succeeded", Endpoint: provider.ChatEndpoint(cfg), Model: cfg.ChatModel})
}

func (h apiHandler) listProviderRoutes(c *gin.Context) {
	items, err := h.deps.Store.ListProviderRoutes(c.Request.Context())
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "provider_routes_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "provider.routes.v1", "items": items})
}

func (h apiHandler) updateProviderRoute(c *gin.Context) {
	var req provider.SaveRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	item, err := h.deps.Store.SaveProviderRoute(c.Request.Context(), c.Param("task_type"), req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "provider_route_save_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "provider.route.v1", "item": item})
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
		Provider:     h.runtimeProvider(c, req.TaskType),
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

func (h apiHandler) runtimeProvider(c *gin.Context, taskType string) *airuntime.ProviderConfig {
	if h.deps.Store == nil {
		return nil
	}
	provider, err := h.deps.Store.RuntimeProviderForTask(c.Request.Context(), taskType)
	if err != nil {
		h.deps.Logger.Warn("resolve runtime provider failed", "task_type", taskType, "error", err)
		return nil
	}
	return provider
}

func (h apiHandler) createInterviewSession(c *gin.Context) {
	var req interview.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	req.UserID = currentUserID(c)
	item, err := h.deps.InterviewService.CreateSession(c.Request.Context(), req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "interview_session_create_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"schema_version": "interview.session.v1", "item": item})
}

func (h apiHandler) getInterviewSession(c *gin.Context) {
	item, err := h.deps.InterviewService.GetSession(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		writeGinError(c, http.StatusNotFound, "interview_session_not_found", err.Error())
		return
	}
	if !canAccessUser(c, item.UserID) {
		writeGinError(c, http.StatusForbidden, "interview_session_forbidden", "session does not belong to current user")
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "interview.session.v1", "item": item})
}

func (h apiHandler) submitInterviewAnswer(c *gin.Context) {
	var req interview.SubmitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	current, err := h.deps.InterviewService.GetSession(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		writeGinError(c, http.StatusNotFound, "interview_session_not_found", err.Error())
		return
	}
	if !canAccessUser(c, current.UserID) {
		writeGinError(c, http.StatusForbidden, "interview_session_forbidden", "session does not belong to current user")
		return
	}
	resp, err := h.deps.InterviewService.SubmitAnswer(c.Request.Context(), c.Param("session_id"), req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "interview_answer_failed", err.Error())
		return
	}
	if accepted, _ := resp["accepted"].(bool); accepted {
		c.JSON(http.StatusAccepted, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h apiHandler) finalizeInterviewSession(c *gin.Context) {
	current, err := h.deps.InterviewService.GetSession(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		writeGinError(c, http.StatusNotFound, "interview_session_not_found", err.Error())
		return
	}
	if !canAccessUser(c, current.UserID) {
		writeGinError(c, http.StatusForbidden, "interview_session_forbidden", "session does not belong to current user")
		return
	}
	item, err := h.deps.InterviewService.Finalize(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "interview_finalize_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "interview.session.v1", "item": item})
}

func (h apiHandler) getInterviewTrace(c *gin.Context) {
	current, err := h.deps.InterviewService.GetSession(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		writeGinError(c, http.StatusNotFound, "interview_session_not_found", err.Error())
		return
	}
	if !canAccessUser(c, current.UserID) {
		writeGinError(c, http.StatusForbidden, "interview_session_forbidden", "session does not belong to current user")
		return
	}
	items, err := h.deps.InterviewService.Trace(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "interview_trace_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "interview.trace.v1", "items": items})
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

func (h apiHandler) deadLetterSummary(c *gin.Context) {
	item, err := h.deps.Store.DeadLetterSummary(c.Request.Context())
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "dead_letter_summary_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "dead_letter.summary.v1", "item": item})
}

func (h apiHandler) listDeadLetters(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := h.deps.Store.ListDeadLetters(c.Request.Context(), c.Query("status"), c.Query("source"), limit)
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "dead_letter_list_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "dead_letter.list.v1", "items": items})
}

func (h apiHandler) getDeadLetter(c *gin.Context) {
	item, ok, err := h.deps.Store.GetDeadLetter(c.Request.Context(), c.Param("dead_letter_id"))
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "dead_letter_get_failed", err.Error())
		return
	}
	if !ok {
		writeGinError(c, http.StatusNotFound, "dead_letter_not_found", "dead_letter_id is not registered")
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "dead_letter.item.v1", "item": item})
}

func (h apiHandler) workerSummary(c *gin.Context) {
	item, err := h.deps.InterviewService.WorkerMetrics(c.Request.Context())
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "worker_metrics_failed", err.Error())
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
