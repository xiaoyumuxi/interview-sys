package contextengine

import (
	"context"
	"fmt"
	"strings"
)

const memoryAdmissionLimit = 5

func (e *Engine) admitMemory(ctx context.Context, req PreviewRequest, now string) ([]ContextItem, *MemoryAdmission) {
	admission := &MemoryAdmission{
		SchemaVersion: "context.memory_admission.v1",
		Enabled:       false,
		UserID:        strings.TrimSpace(req.UserID),
		Limit:         memoryAdmissionLimit,
	}
	if !taskAllowsMemory(req.TaskType) {
		admission.Reasons = append(admission.Reasons, "task_type does not admit long-term memory")
		return nil, admission
	}
	if e.memory == nil {
		admission.Warnings = append(admission.Warnings, "memory_source_unavailable")
		return nil, admission
	}
	if admission.UserID == "" {
		admission.Warnings = append(admission.Warnings, "memory_context_skipped_user_id_missing")
		return nil, admission
	}
	query := memoryQuery(req)
	admission.Enabled = true
	admission.Query = query
	resp, err := e.memory.SearchMemory(ctx, admission.UserID, query, memoryAdmissionLimit)
	if err != nil {
		admission.Warnings = append(admission.Warnings, "memory_search_failed: "+err.Error())
		return nil, admission
	}
	items, reasons := memoryContextItems(req, resp, now, e.memoryTokenBudget())
	admission.Included = len(items)
	admission.CandidateIDs = make([]string, 0, len(items))
	for _, item := range items {
		admission.CandidateIDs = append(admission.CandidateIDs, item.SourceID)
	}
	admission.Reasons = append(admission.Reasons, reasons...)
	return items, admission
}

func taskAllowsMemory(taskType string) bool {
	switch strings.TrimSpace(taskType) {
	case "memory_extraction":
		return false
	default:
		return true
	}
}

func memoryQuery(req PreviewRequest) string {
	if query := strings.TrimSpace(req.MemoryQuery); query != "" {
		return query
	}
	parts := []string{strings.TrimSpace(req.TaskType), strings.TrimSpace(req.SkillID)}
	if req.SessionID != "" {
		parts = append(parts, strings.TrimSpace(req.SessionID))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func memoryContextItems(req PreviewRequest, resp map[string]any, now string, budget int) ([]ContextItem, []string) {
	rawItems, _ := resp["items"].([]any)
	items := make([]ContextItem, 0, len(rawItems))
	reasons := make([]string, 0)
	used := 0
	for _, raw := range rawItems {
		result, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		candidate, ok := result["candidate"].(map[string]any)
		if !ok {
			continue
		}
		if status := stringValue(candidate["status"]); status != "" && status != "approved" {
			continue
		}
		candidateID := stringValue(candidate["candidate_id"])
		content := strings.TrimSpace(stringValue(candidate["content"]))
		if candidateID == "" || content == "" {
			continue
		}
		memoryType := stringValue(candidate["type"])
		topic := stringValue(candidate["topic"])
		confidence := floatValue(candidate["confidence"])
		score := floatValue(result["memory_context_score"])
		if score == 0 {
			score = confidence
		}
		text := formatMemoryContent(memoryType, topic, content, confidence)
		tokens := estimateTokens(text)
		if budget > 0 && used+tokens > budget {
			reasons = append(reasons, "memory token budget reached")
			break
		}
		runtimeReasons := stringSlice(result["reasons"])
		reason := memoryReason(req, runtimeReasons)
		items = append(items, ContextItem{
			ID:         fmt.Sprintf("ctx_mem_%03d", len(items)+1),
			SourceType: "memory_context",
			SourceID:   candidateID,
			TrustLevel: "reviewed",
			Content:    text,
			Tokens:     tokens,
			Score:      score,
			Reason:     reason,
			CreatedAt:  now,
		})
		reasons = append(reasons, candidateID+": "+reason)
		used += tokens
	}
	return items, reasons
}

func (e *Engine) memoryTokenBudget() int {
	if e.tokenBudget <= 0 {
		return 0
	}
	budget := e.tokenBudget / 5
	if budget < 120 {
		return 120
	}
	return budget
}

func formatMemoryContent(memoryType string, topic string, content string, confidence float64) string {
	var builder strings.Builder
	builder.WriteString("Reviewed user memory")
	if memoryType != "" {
		builder.WriteString("\n- type: ")
		builder.WriteString(memoryType)
	}
	if topic != "" {
		builder.WriteString("\n- topic: ")
		builder.WriteString(topic)
	}
	builder.WriteString("\n- content: ")
	builder.WriteString(content)
	if confidence > 0 {
		builder.WriteString(fmt.Sprintf("\n- confidence: %.2f", confidence))
	}
	return builder.String()
}

func memoryReason(req PreviewRequest, runtimeReasons []string) string {
	reason := "approved memory admitted by Go rule"
	if req.TaskType != "" {
		reason += "; task_type=" + req.TaskType
	}
	if req.SkillID != "" {
		reason += "; skill_id=" + req.SkillID
	}
	if len(runtimeReasons) > 0 {
		reason += "; runtime_reason=" + strings.Join(runtimeReasons, ", ")
	}
	return reason
}

func stringValue(value any) string {
	out, _ := value.(string)
	return strings.TrimSpace(out)
}

func floatValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if text := stringValue(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}
