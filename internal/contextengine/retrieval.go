package contextengine

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"ai-interview-platform/internal/skill"
)

const (
	defaultRetrievalLimit = 10
	maxRetrievalLimit     = 30
)

type RetrievalRequest struct {
	TaskType    string `json:"task_type"`
	SkillID     string `json:"skill_id"`
	UserID      string `json:"user_id,omitempty"`
	Query       string `json:"query"`
	SessionID   string `json:"session_id,omitempty"`
	Limit       int    `json:"limit"`
	TokenBudget int    `json:"token_budget"`
	Debug       bool   `json:"debug"`
}

type RetrievalResponse struct {
	SchemaVersion string               `json:"schema_version"`
	Query         string               `json:"query"`
	TokenBudget   int                  `json:"token_budget"`
	Items         []RetrievalItem      `json:"items"`
	Warnings      []string             `json:"warnings,omitempty"`
	Debug         *RetrievalDebugTrace `json:"debug,omitempty"`
}

type RetrievalItem struct {
	ID         string         `json:"id"`
	SourceType string         `json:"source_type"`
	SourceID   string         `json:"source_id"`
	TrustLevel string         `json:"trust_level"`
	Evidence   string         `json:"evidence"`
	Tokens     int            `json:"tokens"`
	Score      float64        `json:"score"`
	Reason     string         `json:"reason"`
	CreatedAt  string         `json:"created_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type RetrievalDebugTrace struct {
	SchemaVersion    string         `json:"schema_version"`
	SourceModes      []string       `json:"source_modes"`
	CandidateCounts  map[string]int `json:"candidate_counts"`
	SelectedCount    int            `json:"selected_count"`
	SkippedForBudget int            `json:"skipped_for_budget"`
	QueryTerms       []string       `json:"query_terms"`
}

type RecentTurn struct {
	TurnID         string         `json:"turn_id"`
	SessionID      string         `json:"session_id"`
	QuestionID     string         `json:"question_id,omitempty"`
	QuestionNumber int            `json:"question_number"`
	AnswerRound    int            `json:"answer_round"`
	UserAnswer     string         `json:"user_answer"`
	Evaluation     map[string]any `json:"evaluation"`
	Score          float64        `json:"score"`
	TurnStatus     string         `json:"turn_status"`
	ErrorText      string         `json:"error_text,omitempty"`
	CreatedAt      string         `json:"created_at"`
}

type RecentHistorySource interface {
	RecentTurns(ctx context.Context, sessionID string, limit int) ([]RecentTurn, error)
}

func (e *Engine) SetRecentHistorySource(source RecentHistorySource) {
	e.recentHistory = source
}

func (e *Engine) Retrieve(ctx context.Context, req RetrievalRequest) (RetrievalResponse, error) {
	if strings.TrimSpace(req.TaskType) == "" {
		return RetrievalResponse{}, errors.New("task_type is required")
	}
	if strings.TrimSpace(req.SkillID) == "" {
		return RetrievalResponse{}, errors.New("skill_id is required")
	}
	skillPack, ok := e.skills.Get(req.SkillID)
	if !ok {
		return RetrievalResponse{}, errors.New("skill_id is not registered")
	}
	query := retrievalQuery(req)
	terms := queryTerms(query)
	limit := retrievalLimit(req.Limit)
	budget := req.TokenBudget
	if budget <= 0 {
		budget = e.tokenBudget
	}
	if budget <= 0 {
		budget = 1200
	}

	now := time.Now().Format(time.RFC3339)
	warnings := []string{"vector_source_unavailable_embedding_not_indexed"}
	counts := map[string]int{}
	candidates := make([]RetrievalItem, 0)

	refItems := referenceRetrievalItems(skillPack.ID, skillPack.References, terms, now)
	counts["skill_reference_full_text"] = countSource(refItems, "skill_reference_full_text")
	counts["skill_reference_summary"] = countSource(refItems, "skill_reference_summary")
	candidates = append(candidates, refItems...)

	recentItems, recentWarnings := e.recentHistoryRetrievalItems(ctx, req, terms)
	warnings = append(warnings, recentWarnings...)
	counts["recent_history"] = len(recentItems)
	candidates = append(candidates, recentItems...)

	memoryItems, memoryWarnings := e.memoryRetrievalItems(ctx, req, query, now)
	warnings = append(warnings, memoryWarnings...)
	counts["approved_memory"] = len(memoryItems)
	candidates = append(candidates, memoryItems...)

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].SourceType < candidates[j].SourceType
		}
		return candidates[i].Score > candidates[j].Score
	})
	items, skipped := selectRetrievalItems(candidates, limit, budget)
	var debug *RetrievalDebugTrace
	if req.Debug {
		debug = &RetrievalDebugTrace{
			SchemaVersion:    "retrieval.debug.v1",
			SourceModes:      []string{"skill_reference_full_text", "skill_reference_summary", "vector", "recent_history", "approved_memory"},
			CandidateCounts:  counts,
			SelectedCount:    len(items),
			SkippedForBudget: skipped,
			QueryTerms:       terms,
		}
	}
	return RetrievalResponse{
		SchemaVersion: "retrieval.harness.v1",
		Query:         query,
		TokenBudget:   budget,
		Items:         items,
		Warnings:      warnings,
		Debug:         debug,
	}, nil
}

func retrievalQuery(req RetrievalRequest) string {
	if query := strings.TrimSpace(req.Query); query != "" {
		return query
	}
	return strings.TrimSpace(strings.Join([]string{req.TaskType, req.SkillID, req.SessionID}, " "))
}

func retrievalLimit(limit int) int {
	if limit <= 0 {
		return defaultRetrievalLimit
	}
	if limit > maxRetrievalLimit {
		return maxRetrievalLimit
	}
	return limit
}

func referenceRetrievalItems(skillID string, refs []skill.Reference, terms []string, now string) []RetrievalItem {
	items := make([]RetrievalItem, 0, len(refs)*2)
	for index, ref := range refs {
		score, reason := keywordScore(terms, ref.Content)
		items = append(items, RetrievalItem{
			ID:         fmt.Sprintf("ret_ref_%03d", index+1),
			SourceType: "skill_reference_full_text",
			SourceID:   ref.SourceID,
			TrustLevel: "trusted",
			Evidence:   ref.Content,
			Tokens:     ref.Tokens,
			Score:      score,
			Reason:     reason + "; full-text fallback over local Skill Pack reference",
			CreatedAt:  now,
			Metadata:   map[string]any{"skill_id": skillID},
		})
		summary := summarizeEvidence(ref.Content, 420)
		summaryScore, summaryReason := keywordScore(terms, summary)
		if summaryScore > 0 {
			summaryScore *= 0.9
		}
		items = append(items, RetrievalItem{
			ID:         fmt.Sprintf("ret_ref_summary_%03d", index+1),
			SourceType: "skill_reference_summary",
			SourceID:   ref.SourceID + "#summary",
			TrustLevel: "trusted",
			Evidence:   summary,
			Tokens:     estimateTokens(summary),
			Score:      summaryScore,
			Reason:     summaryReason + "; summary fallback derived from Skill Pack reference",
			CreatedAt:  now,
			Metadata:   map[string]any{"skill_id": skillID, "reference_id": ref.SourceID},
		})
	}
	return items
}

func (e *Engine) recentHistoryRetrievalItems(ctx context.Context, req RetrievalRequest, terms []string) ([]RetrievalItem, []string) {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil, nil
	}
	if e.recentHistory == nil {
		return nil, []string{"recent_history_source_unavailable"}
	}
	turns, err := e.recentHistory.RecentTurns(ctx, req.SessionID, retrievalLimit(req.Limit))
	if err != nil {
		return nil, []string{"recent_history_search_failed: " + err.Error()}
	}
	items := make([]RetrievalItem, 0, len(turns))
	for index, turn := range turns {
		evidence := formatRecentTurnEvidence(turn)
		score, reason := keywordScore(terms, evidence)
		if score > 0 {
			score += 0.08
			if score > 1 {
				score = 1
			}
		}
		items = append(items, RetrievalItem{
			ID:         fmt.Sprintf("ret_recent_%03d", index+1),
			SourceType: "recent_history",
			SourceID:   turn.TurnID,
			TrustLevel: "trusted",
			Evidence:   evidence,
			Tokens:     estimateTokens(evidence),
			Score:      score,
			Reason:     reason + "; recent interview turn from PostgreSQL",
			CreatedAt:  turn.CreatedAt,
			Metadata: map[string]any{
				"session_id":      turn.SessionID,
				"question_id":     turn.QuestionID,
				"question_number": turn.QuestionNumber,
				"answer_round":    turn.AnswerRound,
				"turn_status":     turn.TurnStatus,
				"score":           turn.Score,
			},
		})
	}
	return items, nil
}

func (e *Engine) memoryRetrievalItems(ctx context.Context, req RetrievalRequest, query string, now string) ([]RetrievalItem, []string) {
	if !taskAllowsMemory(req.TaskType) {
		return nil, nil
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return nil, []string{"approved_memory_skipped_user_id_missing"}
	}
	if e.memory == nil {
		return nil, []string{"approved_memory_source_unavailable"}
	}
	resp, err := e.memory.SearchMemory(ctx, userID, query, retrievalLimit(req.Limit))
	if err != nil {
		return nil, []string{"approved_memory_search_failed: " + err.Error()}
	}
	rawItems, _ := resp["items"].([]any)
	items := make([]RetrievalItem, 0, len(rawItems))
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
		content := stringValue(candidate["content"])
		if candidateID == "" || content == "" {
			continue
		}
		memoryType := stringValue(candidate["type"])
		topic := stringValue(candidate["topic"])
		confidence := floatValue(candidate["confidence"])
		evidence := formatMemoryContent(memoryType, topic, content, confidence)
		score := floatValue(result["memory_context_score"])
		if score == 0 {
			score = confidence
		}
		items = append(items, RetrievalItem{
			ID:         fmt.Sprintf("ret_mem_%03d", len(items)+1),
			SourceType: "approved_memory",
			SourceID:   candidateID,
			TrustLevel: "reviewed",
			Evidence:   evidence,
			Tokens:     estimateTokens(evidence),
			Score:      score,
			Reason:     "approved memory from runtime search; " + strings.Join(stringSlice(result["reasons"]), ", "),
			CreatedAt:  now,
			Metadata: map[string]any{
				"user_id":    userID,
				"type":       memoryType,
				"topic":      topic,
				"confidence": confidence,
			},
		})
	}
	return items, nil
}

func selectRetrievalItems(candidates []RetrievalItem, limit int, budget int) ([]RetrievalItem, int) {
	selected := make([]RetrievalItem, 0, limit)
	used := 0
	skipped := 0
	for _, item := range candidates {
		if item.Score <= 0 {
			continue
		}
		if len(selected) >= limit {
			break
		}
		if budget > 0 && used+item.Tokens > budget {
			skipped++
			continue
		}
		selected = append(selected, item)
		used += item.Tokens
	}
	return selected, skipped
}

func formatRecentTurnEvidence(turn RecentTurn) string {
	var builder strings.Builder
	builder.WriteString("Recent interview turn")
	if turn.QuestionID != "" {
		builder.WriteString("\n- question_id: ")
		builder.WriteString(turn.QuestionID)
	}
	builder.WriteString(fmt.Sprintf("\n- question_number: %d", turn.QuestionNumber))
	builder.WriteString(fmt.Sprintf("\n- answer_round: %d", turn.AnswerRound))
	builder.WriteString("\n- status: ")
	builder.WriteString(turn.TurnStatus)
	if turn.UserAnswer != "" {
		builder.WriteString("\n- answer: ")
		builder.WriteString(turn.UserAnswer)
	}
	if turn.Score > 0 {
		builder.WriteString(fmt.Sprintf("\n- score: %.2f", turn.Score))
	}
	if len(turn.Evaluation) > 0 {
		builder.WriteString("\n- evaluation: ")
		builder.WriteString(compactMapText(turn.Evaluation))
	}
	if turn.ErrorText != "" {
		builder.WriteString("\n- error: ")
		builder.WriteString(turn.ErrorText)
	}
	return builder.String()
}

func compactMapText(input map[string]any) string {
	parts := make([]string, 0, len(input))
	for key, value := range input {
		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func summarizeEvidence(content string, maxRunes int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	paragraphs := strings.Split(content, "\n\n")
	summary := strings.TrimSpace(paragraphs[0])
	runes := []rune(summary)
	if len(runes) <= maxRunes {
		return summary
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "..."
}

func countSource(items []RetrievalItem, sourceType string) int {
	count := 0
	for _, item := range items {
		if item.SourceType == sourceType {
			count++
		}
	}
	return count
}

var queryTermPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

func queryTerms(query string) []string {
	matches := queryTermPattern.FindAllString(strings.ToLower(query), -1)
	seen := map[string]bool{}
	terms := make([]string, 0, len(matches))
	for _, match := range matches {
		match = strings.TrimSpace(match)
		if match == "" || seen[match] {
			continue
		}
		seen[match] = true
		terms = append(terms, match)
	}
	return terms
}

func keywordScore(terms []string, content string) (float64, string) {
	if strings.TrimSpace(content) == "" {
		return 0, "empty evidence"
	}
	if len(terms) == 0 {
		return 0.2, "no query terms; low-priority fallback candidate"
	}
	text := strings.ToLower(content)
	matched := make([]string, 0, len(terms))
	for _, term := range terms {
		if strings.Contains(text, term) {
			matched = append(matched, term)
		}
	}
	if len(matched) == 0 {
		return 0, "no query term matched"
	}
	score := 0.25 + (float64(len(matched))/float64(len(terms)))*0.65
	if score > 1 {
		score = 1
	}
	return score, "matched query terms: " + strings.Join(matched, ", ")
}
