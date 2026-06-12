package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	payload, err := json.Marshal(req)
	if err != nil {
		return TaskResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/runtime/tasks", bytes.NewReader(payload))
	if err != nil {
		return TaskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return TaskResponse{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TaskResponse{}, fmt.Errorf("runtime returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out TaskResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return TaskResponse{}, err
	}
	return out, nil
}
