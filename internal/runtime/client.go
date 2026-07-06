package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-interview-platform/internal/contextengine"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

type TaskRequest struct {
	TaskType     string                      `json:"task_type"`
	Provider     *ProviderConfig             `json:"provider,omitempty"`
	ContextItems []contextengine.ContextItem `json:"context_items"`
	UserInput    string                      `json:"user_input"`
	DryRun       bool                        `json:"dry_run"`
}

type ProviderConfig struct {
	ProviderType     string `json:"provider_type"`
	BaseURL          string `json:"base_url"`
	ChatEndpointPath string `json:"chat_endpoint_path"`
	Model            string `json:"model"`
	APIKey           string `json:"api_key"`
	SupportsJSON     bool   `json:"supports_json"`
}

type TaskResponse struct {
	SchemaVersion string         `json:"schema_version"`
	TaskType      string         `json:"task_type"`
	OK            bool           `json:"ok"`
	Output        map[string]any `json:"output"`
	RawOutput     string         `json:"raw_output"`
	Warnings      []string       `json:"warnings"`
	Trace         map[string]any `json:"trace"`
}

func (c *Client) RunTask(ctx context.Context, req TaskRequest) (TaskResponse, error) {
	body, err := c.doJSON(ctx, http.MethodPost, "/api/runtime/tasks", nil, req)
	if err != nil {
		return TaskResponse{}, err
	}
	var out TaskResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return TaskResponse{}, err
	}
	return out, nil
}

type MemoryCandidateCreateRequest struct {
	UserID          string           `json:"user_id"`
	Type            string           `json:"type"`
	Topic           string           `json:"topic"`
	Content         string           `json:"content"`
	Evidence        []map[string]any `json:"evidence,omitempty"`
	Confidence      float64          `json:"confidence"`
	SourceSessionID string           `json:"source_session_id"`
	SourceAnswerID  string           `json:"source_answer_id"`
	ConflictsWith   []string         `json:"conflicts_with,omitempty"`
}

type MemoryReviewRequest struct {
	ReviewNote string `json:"review_note"`
}

type MemoryCandidateEditRequest struct {
	Type          string            `json:"type"`
	Topic         string            `json:"topic"`
	Content       string            `json:"content"`
	Evidence      *[]map[string]any `json:"evidence,omitempty"`
	Confidence    *float64          `json:"confidence,omitempty"`
	ConflictsWith []string          `json:"conflicts_with,omitempty"`
	ReviewNote    string            `json:"review_note"`
}

func (c *Client) ListMemoryCandidates(ctx context.Context, userID string, status string, limit int) (map[string]any, error) {
	values := url.Values{}
	values.Set("user_id", userID)
	values.Set("status", status)
	values.Set("limit", fmt.Sprintf("%d", limit))
	return c.doRuntimeMap(ctx, http.MethodGet, "/api/runtime/memory/candidates", values, nil)
}

func (c *Client) CreateMemoryCandidate(ctx context.Context, req MemoryCandidateCreateRequest) (map[string]any, error) {
	return c.doRuntimeMap(ctx, http.MethodPost, "/api/runtime/memory/candidates", nil, req)
}

func (c *Client) ApproveMemoryCandidate(ctx context.Context, candidateID string, req MemoryReviewRequest) (map[string]any, error) {
	return c.doRuntimeMap(ctx, http.MethodPost, "/api/runtime/memory/candidates/"+url.PathEscape(candidateID)+"/approve", nil, req)
}

func (c *Client) RejectMemoryCandidate(ctx context.Context, candidateID string, req MemoryReviewRequest) (map[string]any, error) {
	return c.doRuntimeMap(ctx, http.MethodPost, "/api/runtime/memory/candidates/"+url.PathEscape(candidateID)+"/reject", nil, req)
}

func (c *Client) EditMemoryCandidate(ctx context.Context, candidateID string, req MemoryCandidateEditRequest) (map[string]any, error) {
	return c.doRuntimeMap(ctx, http.MethodPost, "/api/runtime/memory/candidates/"+url.PathEscape(candidateID)+"/edit", nil, req)
}

func (c *Client) MemoryProfile(ctx context.Context, userID string) (map[string]any, error) {
	values := url.Values{}
	values.Set("user_id", userID)
	return c.doRuntimeMap(ctx, http.MethodGet, "/api/runtime/memory/profile", values, nil)
}

func (c *Client) SearchMemory(ctx context.Context, userID string, query string, limit int) (map[string]any, error) {
	values := url.Values{}
	values.Set("user_id", userID)
	values.Set("q", query)
	values.Set("limit", fmt.Sprintf("%d", limit))
	return c.doRuntimeMap(ctx, http.MethodGet, "/api/runtime/memory/search", values, nil)
}

func (c *Client) DueReviews(ctx context.Context, userID string, limit int) (map[string]any, error) {
	values := url.Values{}
	values.Set("user_id", userID)
	values.Set("limit", fmt.Sprintf("%d", limit))
	return c.doRuntimeMap(ctx, http.MethodGet, "/api/runtime/reviews/due", values, nil)
}

func (c *Client) doRuntimeMap(ctx context.Context, method string, path string, query url.Values, in any) (map[string]any, error) {
	body, err := c.doJSON(ctx, method, path, query, in)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) doJSON(ctx context.Context, method string, path string, query url.Values, in any) ([]byte, error) {
	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(payload)
	}
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if in != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("runtime returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}
