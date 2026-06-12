package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

type Type string

const (
	TypeDeepSeek         Type = "deepseek"
	TypeOpenAICompatible Type = "openai_compatible"
	TypeClaude           Type = "claude"
	TypeEmbedding        Type = "embedding"
)

type Config struct {
	ProviderID        string `json:"provider_id"`
	ProviderType      Type   `json:"provider_type"`
	BaseURL           string `json:"base_url"`
	ChatModel         string `json:"chat_model,omitempty"`
	EmbeddingModel    string `json:"embedding_model,omitempty"`
	SupportsStreaming bool   `json:"supports_streaming"`
	SupportsJSON      bool   `json:"supports_json"`
	Enabled           bool   `json:"enabled"`
}

type Registry struct {
	items []Config
}

func NewRegistry() *Registry {
	items := []Config{
		{
			ProviderID:        "deepseek-default",
			ProviderType:      TypeDeepSeek,
			BaseURL:           env("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
			ChatModel:         env("DEEPSEEK_CHAT_MODEL", "deepseek-chat"),
			SupportsStreaming: true,
			SupportsJSON:      true,
			Enabled:           env("DEEPSEEK_API_KEY", "") != "",
		},
		{
			ProviderID:        "openai-compatible-default",
			ProviderType:      TypeOpenAICompatible,
			BaseURL:           env("OPENAI_COMPATIBLE_BASE_URL", ""),
			ChatModel:         env("OPENAI_COMPATIBLE_CHAT_MODEL", ""),
			SupportsStreaming: true,
			SupportsJSON:      true,
			Enabled:           env("OPENAI_COMPATIBLE_API_KEY", "") != "" && env("OPENAI_COMPATIBLE_BASE_URL", "") != "",
		},
		{
			ProviderID:     "embedding-default",
			ProviderType:   TypeEmbedding,
			BaseURL:        env("EMBEDDING_BASE_URL", ""),
			EmbeddingModel: env("EMBEDDING_MODEL", ""),
			SupportsJSON:   true,
			Enabled:        env("EMBEDDING_API_KEY", "") != "" && env("EMBEDDING_BASE_URL", "") != "",
		},
	}
	return &Registry{items: items}
}

func (r *Registry) List() []Config {
	return append([]Config(nil), r.items...)
}

type TestRequest struct {
	ProviderID string `json:"provider_id"`
	Prompt     string `json:"prompt"`
}

type TestResponse struct {
	SchemaVersion string `json:"schema_version"`
	ProviderID    string `json:"provider_id"`
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
}

func (r *Registry) Test(ctx context.Context, req TestRequest) TestResponse {
	cfg, ok := r.find(req.ProviderID)
	if !ok {
		return TestResponse{SchemaVersion: "provider.test.v1", ProviderID: req.ProviderID, OK: false, Message: "provider not found"}
	}
	if !cfg.Enabled {
		return TestResponse{SchemaVersion: "provider.test.v1", ProviderID: cfg.ProviderID, OK: false, Message: "provider is disabled or missing credentials"}
	}
	if cfg.ProviderType == TypeEmbedding {
		return TestResponse{SchemaVersion: "provider.test.v1", ProviderID: cfg.ProviderID, OK: true, Message: "embedding provider config is present"}
	}
	if err := testChat(ctx, cfg, req.Prompt); err != nil {
		return TestResponse{SchemaVersion: "provider.test.v1", ProviderID: cfg.ProviderID, OK: false, Message: err.Error()}
	}
	return TestResponse{SchemaVersion: "provider.test.v1", ProviderID: cfg.ProviderID, OK: true, Message: "chat completion request succeeded"}
}

func (r *Registry) find(providerID string) (Config, bool) {
	for _, item := range r.items {
		if item.ProviderID == providerID {
			return item, true
		}
	}
	return Config{}, false
}

func testChat(ctx context.Context, cfg Config, prompt string) error {
	apiKey := apiKeyFor(cfg.ProviderType)
	if apiKey == "" {
		return errors.New("missing api key")
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = "Reply with ok."
	}

	body := map[string]any{
		"model": cfg.ChatModel,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a provider connectivity checker."},
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("provider returned " + response.Status)
	}
	return nil
}

func apiKeyFor(providerType Type) string {
	switch providerType {
	case TypeDeepSeek:
		return env("DEEPSEEK_API_KEY", "")
	case TypeOpenAICompatible:
		return env("OPENAI_COMPATIBLE_API_KEY", "")
	default:
		return ""
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
