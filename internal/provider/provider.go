package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	ChatEndpointPath  string `json:"chat_endpoint_path,omitempty"`
	ChatModel         string `json:"chat_model,omitempty"`
	EmbeddingModel    string `json:"embedding_model,omitempty"`
	SupportsStreaming bool   `json:"supports_streaming"`
	SupportsJSON      bool   `json:"supports_json"`
	Enabled           bool   `json:"enabled"`
	APIKeyConfigured  bool   `json:"api_key_configured"`
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
			ChatEndpointPath:  env("DEEPSEEK_CHAT_ENDPOINT_PATH", "/chat/completions"),
			ChatModel:         env("DEEPSEEK_CHAT_MODEL", "deepseek-v4-flash"),
			SupportsStreaming: true,
			SupportsJSON:      true,
			Enabled:           env("DEEPSEEK_API_KEY", "") != "",
			APIKeyConfigured:  env("DEEPSEEK_API_KEY", "") != "",
		},
		{
			ProviderID:        "openai-compatible-default",
			ProviderType:      TypeOpenAICompatible,
			BaseURL:           env("OPENAI_COMPATIBLE_BASE_URL", "https://api.openai.com/v1"),
			ChatEndpointPath:  env("OPENAI_COMPATIBLE_CHAT_ENDPOINT_PATH", "/chat/completions"),
			ChatModel:         env("OPENAI_COMPATIBLE_CHAT_MODEL", env("OPENAI_CHAT_MODEL", "")),
			SupportsStreaming: true,
			SupportsJSON:      true,
			Enabled:           openAICompatibleAPIKey() != "" && env("OPENAI_COMPATIBLE_CHAT_MODEL", env("OPENAI_CHAT_MODEL", "")) != "",
			APIKeyConfigured:  openAICompatibleAPIKey() != "",
		},
		{
			ProviderID:       "embedding-default",
			ProviderType:     TypeEmbedding,
			BaseURL:          env("EMBEDDING_BASE_URL", ""),
			EmbeddingModel:   env("EMBEDDING_MODEL", ""),
			SupportsJSON:     true,
			Enabled:          env("EMBEDDING_API_KEY", "") != "" && env("EMBEDDING_BASE_URL", "") != "",
			APIKeyConfigured: env("EMBEDDING_API_KEY", "") != "",
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
	Endpoint      string `json:"endpoint,omitempty"`
	Model         string `json:"model,omitempty"`
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
		return TestResponse{
			SchemaVersion: "provider.test.v1",
			ProviderID:    cfg.ProviderID,
			OK:            false,
			Message:       err.Error(),
			Endpoint:      chatEndpoint(cfg),
			Model:         cfg.ChatModel,
		}
	}
	return TestResponse{
		SchemaVersion: "provider.test.v1",
		ProviderID:    cfg.ProviderID,
		OK:            true,
		Message:       "chat completion request succeeded",
		Endpoint:      chatEndpoint(cfg),
		Model:         cfg.ChatModel,
	}
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
		"stream":      false,
		"temperature": 0,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	endpoint := chatEndpoint(cfg)
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
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("provider returned %s: %s", response.Status, message)
	}
	return nil
}

func apiKeyFor(providerType Type) string {
	switch providerType {
	case TypeDeepSeek:
		return env("DEEPSEEK_API_KEY", "")
	case TypeOpenAICompatible:
		return openAICompatibleAPIKey()
	default:
		return ""
	}
}

func openAICompatibleAPIKey() string {
	return env("OPENAI_COMPATIBLE_API_KEY", env("OPENAI_API_KEY", ""))
}

func chatEndpoint(cfg Config) string {
	path := strings.TrimSpace(cfg.ChatEndpointPath)
	if path == "" {
		path = "/chat/completions"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(cfg.BaseURL, "/") + path
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
