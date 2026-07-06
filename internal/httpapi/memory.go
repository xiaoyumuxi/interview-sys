package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	airuntime "ai-interview-platform/internal/runtime"
	"ai-interview-platform/internal/store"

	"github.com/gin-gonic/gin"
)

func (h apiHandler) listMemoryCandidates(c *gin.Context) {
	userID := memoryTargetUserID(c, c.Query("user_id"))
	if userID == "" {
		writeGinError(c, http.StatusForbidden, "memory_user_required", "user_id is required")
		return
	}
	limit := queryLimit(c, "limit", 100, 200)
	resp, err := h.deps.RuntimeClient.ListMemoryCandidates(c.Request.Context(), userID, c.Query("status"), limit)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version":   "memory.candidates.v1",
		"runtime_response": resp,
	})
}

func (h apiHandler) createMemoryCandidate(c *gin.Context) {
	var req airuntime.MemoryCandidateCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	req.UserID = memoryTargetUserID(c, req.UserID)
	if req.UserID == "" {
		writeGinError(c, http.StatusForbidden, "memory_user_required", "user_id is required")
		return
	}
	resp, err := h.deps.RuntimeClient.CreateMemoryCandidate(c.Request.Context(), req)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return
	}
	traceID := h.insertMemoryTrace(c, "memory.candidate.create", req, resp)
	c.JSON(http.StatusCreated, gin.H{
		"schema_version":   "memory.candidate.v1",
		"trace_id":         traceID,
		"runtime_response": resp,
	})
}

func (h apiHandler) approveMemoryCandidate(c *gin.Context) {
	var req airuntime.MemoryReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	candidateID := strings.TrimSpace(c.Param("candidate_id"))
	if ok := h.ensureMemoryCandidateAccess(c, candidateID); !ok {
		return
	}
	resp, err := h.deps.RuntimeClient.ApproveMemoryCandidate(c.Request.Context(), candidateID, req)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return
	}
	traceID := h.insertMemoryTrace(c, "memory.candidate.approve", gin.H{"candidate_id": candidateID, "request": req}, resp)
	c.JSON(http.StatusOK, gin.H{
		"schema_version":   "memory.candidate.v1",
		"trace_id":         traceID,
		"runtime_response": resp,
	})
}

func (h apiHandler) rejectMemoryCandidate(c *gin.Context) {
	var req airuntime.MemoryReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	candidateID := strings.TrimSpace(c.Param("candidate_id"))
	if ok := h.ensureMemoryCandidateAccess(c, candidateID); !ok {
		return
	}
	resp, err := h.deps.RuntimeClient.RejectMemoryCandidate(c.Request.Context(), candidateID, req)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return
	}
	traceID := h.insertMemoryTrace(c, "memory.candidate.reject", gin.H{"candidate_id": candidateID, "request": req}, resp)
	c.JSON(http.StatusOK, gin.H{
		"schema_version":   "memory.candidate.v1",
		"trace_id":         traceID,
		"runtime_response": resp,
	})
}

func (h apiHandler) editMemoryCandidate(c *gin.Context) {
	var req airuntime.MemoryCandidateEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	candidateID := strings.TrimSpace(c.Param("candidate_id"))
	if ok := h.ensureMemoryCandidateAccess(c, candidateID); !ok {
		return
	}
	resp, err := h.deps.RuntimeClient.EditMemoryCandidate(c.Request.Context(), candidateID, req)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return
	}
	traceID := h.insertMemoryTrace(c, "memory.candidate.edit", gin.H{"candidate_id": candidateID, "request": req}, resp)
	c.JSON(http.StatusOK, gin.H{
		"schema_version":   "memory.candidate.v1",
		"trace_id":         traceID,
		"runtime_response": resp,
	})
}

func (h apiHandler) getMemoryProfile(c *gin.Context) {
	userID := memoryTargetUserID(c, c.Query("user_id"))
	if userID == "" {
		writeGinError(c, http.StatusForbidden, "memory_user_required", "user_id is required")
		return
	}
	resp, err := h.deps.RuntimeClient.MemoryProfile(c.Request.Context(), userID)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version":   "memory.profile.v1",
		"runtime_response": resp,
	})
}

func (h apiHandler) searchMemory(c *gin.Context) {
	userID := memoryTargetUserID(c, c.Query("user_id"))
	if userID == "" {
		writeGinError(c, http.StatusForbidden, "memory_user_required", "user_id is required")
		return
	}
	limit := queryLimit(c, "limit", 20, 50)
	resp, err := h.deps.RuntimeClient.SearchMemory(c.Request.Context(), userID, c.Query("q"), limit)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version":   "memory.search.v1",
		"runtime_response": resp,
	})
}

func (h apiHandler) listDueMemoryReviews(c *gin.Context) {
	userID := memoryTargetUserID(c, c.Query("user_id"))
	if userID == "" {
		writeGinError(c, http.StatusForbidden, "memory_user_required", "user_id is required")
		return
	}
	limit := queryLimit(c, "limit", 50, 100)
	resp, err := h.deps.RuntimeClient.DueReviews(c.Request.Context(), userID, limit)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version":   "memory.reviews.due.v1",
		"runtime_response": resp,
	})
}

func (h apiHandler) ensureMemoryCandidateAccess(c *gin.Context, candidateID string) bool {
	if candidateID == "" {
		writeGinError(c, http.StatusBadRequest, "memory_candidate_required", "candidate_id is required")
		return false
	}
	userID := memoryTargetUserID(c, c.Query("user_id"))
	resp, err := h.deps.RuntimeClient.ListMemoryCandidates(c.Request.Context(), userID, "", 200)
	if err != nil {
		writeGinError(c, http.StatusBadGateway, "runtime_memory_failed", err.Error())
		return false
	}
	candidate, ok := findRuntimeCandidate(resp, candidateID)
	if !ok {
		writeGinError(c, http.StatusNotFound, "memory_candidate_not_found", "memory candidate is not accessible")
		return false
	}
	ownerID, _ := candidate["user_id"].(string)
	if !canAccessUser(c, ownerID) {
		writeGinError(c, http.StatusForbidden, "memory_candidate_forbidden", "memory candidate does not belong to current user")
		return false
	}
	return true
}

func (h apiHandler) insertMemoryTrace(c *gin.Context, taskType string, input any, output any) string {
	traceID := store.NewID("trace")
	if h.deps.Store == nil {
		return traceID
	}
	if err := h.deps.Store.InsertAgentTrace(c.Request.Context(), store.TraceRecord{
		TraceID:      traceID,
		TaskType:     taskType,
		SkillID:      "",
		Input:        input,
		ContextItems: nil,
		Output:       output,
	}); err != nil && h.deps.Logger != nil {
		h.deps.Logger.Warn("insert memory trace failed", "task_type", taskType, "error", err)
	}
	return traceID
}

func memoryTargetUserID(c *gin.Context, requested string) string {
	requested = strings.TrimSpace(requested)
	if currentRole(c) == "root" && requested != "" {
		return requested
	}
	return currentUserID(c)
}

func findRuntimeCandidate(resp map[string]any, candidateID string) (map[string]any, bool) {
	items, _ := resp["items"].([]any)
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := item["candidate_id"].(string); id == candidateID {
			return item, true
		}
	}
	return nil, false
}

func queryLimit(c *gin.Context, key string, defaultValue int, maxValue int) int {
	limit, err := strconv.Atoi(c.DefaultQuery(key, strconv.Itoa(defaultValue)))
	if err != nil || limit <= 0 {
		return defaultValue
	}
	if limit > maxValue {
		return maxValue
	}
	return limit
}
