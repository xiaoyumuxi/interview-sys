package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"ai-interview-platform/internal/evalharness"

	"github.com/gin-gonic/gin"
)

func (h apiHandler) listEvaluationCases(c *gin.Context) {
	if h.deps.Evaluation == nil {
		writeGinError(c, http.StatusServiceUnavailable, "evaluation_unavailable", "evaluation harness is not configured")
		return
	}
	items, err := h.deps.Evaluation.ListCases(c.Request.Context(), c.Query("suite"), c.Query("task_type"), intQuery(c, "limit", 100))
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "evaluation_cases_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version": "evaluation.case.list.v1",
		"items":          items,
	})
}

func (h apiHandler) createEvaluationCase(c *gin.Context) {
	if h.deps.Evaluation == nil {
		writeGinError(c, http.StatusServiceUnavailable, "evaluation_unavailable", "evaluation harness is not configured")
		return
	}
	var req evalharness.SaveCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	item, err := h.deps.Evaluation.SaveCase(c.Request.Context(), req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "evaluation_case_save_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"schema_version": "evaluation.case.v1", "item": item})
}

func (h apiHandler) getEvaluationCase(c *gin.Context) {
	if h.deps.Evaluation == nil {
		writeGinError(c, http.StatusServiceUnavailable, "evaluation_unavailable", "evaluation harness is not configured")
		return
	}
	item, ok, err := h.deps.Evaluation.GetCase(c.Request.Context(), c.Param("case_id"))
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "evaluation_case_get_failed", err.Error())
		return
	}
	if !ok {
		writeGinError(c, http.StatusNotFound, "evaluation_case_not_found", "case_id is not registered")
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "evaluation.case.v1", "item": item})
}

type evaluationRunRequest struct {
	DryRun bool `json:"dry_run"`
}

func (h apiHandler) runEvaluationCase(c *gin.Context) {
	if h.deps.Evaluation == nil {
		writeGinError(c, http.StatusServiceUnavailable, "evaluation_unavailable", "evaluation harness is not configured")
		return
	}
	var req evaluationRunRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result, err := h.deps.Evaluation.RunCase(c.Request.Context(), evalharness.RunCaseRequest{
		CaseID: c.Param("case_id"),
		UserID: currentUserID(c),
		DryRun: req.DryRun,
	})
	if err != nil {
		if errors.Is(err, evalharness.ErrCaseNotFound) {
			writeGinError(c, http.StatusNotFound, "evaluation_case_not_found", "case_id is not registered")
			return
		}
		writeGinError(c, http.StatusInternalServerError, "evaluation_run_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h apiHandler) listEvaluationRuns(c *gin.Context) {
	if h.deps.Evaluation == nil {
		writeGinError(c, http.StatusServiceUnavailable, "evaluation_unavailable", "evaluation harness is not configured")
		return
	}
	items, err := h.deps.Evaluation.ListRuns(c.Request.Context(), c.Query("case_id"), c.Query("task_type"), intQuery(c, "limit", 100))
	if err != nil {
		writeGinError(c, http.StatusInternalServerError, "evaluation_runs_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version": "evaluation.run.list.v1",
		"items":          items,
	})
}

func intQuery(c *gin.Context, key string, fallback int) int {
	value := c.Query(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
