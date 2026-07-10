package contextengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-interview-platform/internal/skill"
)

type PreviewRequest struct {
	TaskType    string `json:"task_type"`
	SkillID     string `json:"skill_id"`
	UserID      string `json:"user_id,omitempty"`
	MemoryQuery string `json:"memory_query,omitempty"`
	ResumeID    string `json:"resume_id,omitempty"`
	JDID        string `json:"jd_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}

type PreviewResponse struct {
	SchemaVersion      string           `json:"schema_version"`
	Recipe             string           `json:"recipe"`
	TokenBudget        int              `json:"token_budget"`
	Items              []ContextItem    `json:"items"`
	FinalPromptPreview string           `json:"final_prompt_preview"`
	Warnings           []string         `json:"warnings"`
	MemoryAdmission    *MemoryAdmission `json:"memory_admission,omitempty"`
}

type ContextItem struct {
	ID         string  `json:"id"`
	SourceType string  `json:"source_type"`
	SourceID   string  `json:"source_id"`
	TrustLevel string  `json:"trust_level"`
	Content    string  `json:"content"`
	Tokens     int     `json:"tokens"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
	CreatedAt  string  `json:"created_at"`
}

type MemoryAdmission struct {
	SchemaVersion string   `json:"schema_version"`
	Enabled       bool     `json:"enabled"`
	UserID        string   `json:"user_id,omitempty"`
	Query         string   `json:"query,omitempty"`
	Limit         int      `json:"limit"`
	Included      int      `json:"included"`
	CandidateIDs  []string `json:"candidate_ids,omitempty"`
	Reasons       []string `json:"reasons,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type MemorySource interface {
	SearchMemory(ctx context.Context, userID string, query string, limit int) (map[string]any, error)
}

type Engine struct {
	tokenBudget   int
	skills        *skill.Registry
	memory        MemorySource
	recentHistory RecentHistorySource
}

func New(tokenBudget int, skills *skill.Registry) *Engine {
	return &Engine{tokenBudget: tokenBudget, skills: skills}
}

func (e *Engine) SetMemorySource(memory MemorySource) {
	e.memory = memory
}

func (e *Engine) Preview(ctx context.Context, req PreviewRequest) (PreviewResponse, error) {
	if strings.TrimSpace(req.TaskType) == "" {
		return PreviewResponse{}, errors.New("task_type is required")
	}
	skillPack, ok := e.skills.Get(req.SkillID)
	if !ok {
		return PreviewResponse{}, errors.New("skill_id is not registered")
	}

	now := time.Now().Format(time.RFC3339)
	items := []ContextItem{
		{
			ID:         "ctx_system_001",
			SourceType: "system_context",
			SourceID:   "system/context-rules",
			TrustLevel: "trusted",
			Content:    "AI outputs must be structured, traceable, and must not write unreviewed memory into long-term profile.",
			Tokens:     28,
			Score:      1,
			Reason:     "global rule from backend plan",
			CreatedAt:  now,
		},
		{
			ID:         "ctx_skill_001",
			SourceType: "skill_context",
			SourceID:   skillPack.ID + "/SKILL.md",
			TrustLevel: "trusted",
			Content:    skillPack.Instructions,
			Tokens:     estimateTokens(skillPack.Instructions),
			Score:      0.95,
			Reason:     "requested skill_id matches registered Skill Pack",
			CreatedAt:  now,
		},
		{
			ID:         "ctx_task_001",
			SourceType: "task_context",
			SourceID:   "task/" + req.TaskType,
			TrustLevel: "trusted",
			Content:    taskInstruction(req.TaskType),
			Tokens:     80,
			Score:      0.9,
			Reason:     "task_type selects context recipe and task instruction",
			CreatedAt:  now,
		},
	}
	warnings := []string{"embedding_unavailable_fallback_to_skill_and_keyword"}
	memoryItems, admission := e.admitMemory(ctx, req, now)
	items = append(items, memoryItems...)
	warnings = append(warnings, admission.Warnings...)

	for index, ref := range skillPack.References {
		items = append(items, ContextItem{
			ID:         fmt.Sprintf("ctx_ref_%03d", index+1),
			SourceType: "skill_reference",
			SourceID:   ref.SourceID,
			TrustLevel: "trusted",
			Content:    ref.Content,
			Tokens:     ref.Tokens,
			Score:      0.8 - float64(index)*0.03,
			Reason:     "loaded from Skill Pack reference and available for keyword/full-text fallback",
			CreatedAt:  now,
		})
	}

	return PreviewResponse{
		SchemaVersion:      "context.preview.v1",
		Recipe:             recipeFor(req.TaskType),
		TokenBudget:        e.tokenBudget,
		Items:              items,
		FinalPromptPreview: packPrompt(items, e.tokenBudget),
		Warnings:           warnings,
		MemoryAdmission:    admission,
	}, nil
}

func recipeFor(taskType string) string {
	switch taskType {
	case "answer_evaluation":
		return "answer_evaluation_v1"
	case "follow_up_decision":
		return "follow_up_decision_v1"
	case "summary":
		return "summary_v1"
	case "memory_extraction":
		return "memory_extraction_v1"
	default:
		return "question_generation_v1"
	}
}

func taskInstruction(taskType string) string {
	switch taskType {
	case "answer_evaluation":
		return "Evaluate the answer against the skill rubric, cite evidence, and produce structured scoring output."
	case "follow_up_decision":
		return "Decide whether to ask a follow-up based on the current answer, missing points, and interview progress."
	case "memory_extraction":
		return "Extract candidate memories only. Do not update the user profile; every item must remain pending review."
	case "summary":
		return "Summarize the session with traceable evidence and distinguish facts from model judgments."
	default:
		return "Generate one interview question using the selected skill, references, and reviewed context only."
	}
}

func packPrompt(items []ContextItem, budget int) string {
	var builder strings.Builder
	used := 0
	for _, item := range items {
		if used+item.Tokens > budget {
			continue
		}
		builder.WriteString("## ")
		builder.WriteString(item.SourceType)
		builder.WriteString(" ")
		builder.WriteString(item.SourceID)
		builder.WriteString("\n")
		builder.WriteString(item.Content)
		builder.WriteString("\n\n")
		used += item.Tokens
	}
	return builder.String()
}

func estimateTokens(content string) int {
	words := len(strings.Fields(content))
	if words == 0 && content != "" {
		return len([]rune(content)) / 2
	}
	return words*2 + len([]rune(content))/8
}
