package provider

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	TypeEmbedding        Type = "embedding"
)

type KeySource string

const (
	KeySourceEnvRef      KeySource = "env_ref"
	KeySourceDBEncrypted KeySource = "db_encrypted"
	KeySourceNone        KeySource = "none"
)

type Config struct {
	ProviderID        string    `json:"provider_id"`
	ProviderType      Type      `json:"provider_type"`
	BaseURL           string    `json:"base_url"`
	ChatEndpointPath  string    `json:"chat_endpoint_path,omitempty"`
	APIKeyRef         string    `json:"api_key_ref,omitempty"`
	APIKeySource      KeySource `json:"api_key_source"`
	ChatModel         string    `json:"chat_model,omitempty"`
	EmbeddingModel    string    `json:"embedding_model,omitempty"`
	SupportsStreaming bool      `json:"supports_streaming"`
	SupportsJSON      bool      `json:"supports_json"`
	Enabled           bool      `json:"enabled"`
	APIKeyConfigured  bool      `json:"api_key_configured"`
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
			APIKeyRef:         "DEEPSEEK_API_KEY",
			APIKeySource:      KeySourceEnvRef,
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
			APIKeyRef:         "OPENAI_COMPATIBLE_API_KEY",
			APIKeySource:      KeySourceEnvRef,
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
			APIKeyRef:        "EMBEDDING_API_KEY",
			APIKeySource:     KeySourceEnvRef,
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

type SaveRequest struct {
	ProviderID        string `json:"provider_id"`
	ProviderType      Type   `json:"provider_type"`
	BaseURL           string `json:"base_url"`
	ChatEndpointPath  string `json:"chat_endpoint_path"`
	APIKeyRef         string `json:"api_key_ref"`
	APIKey            string `json:"api_key"`
	ClearAPIKey       bool   `json:"clear_api_key"`
	ChatModel         string `json:"chat_model"`
	EmbeddingModel    string `json:"embedding_model"`
	SupportsStreaming bool   `json:"supports_streaming"`
	SupportsJSON      bool   `json:"supports_json"`
	Enabled           bool   `json:"enabled"`
}

type Route struct {
	TaskType           string `json:"task_type"`
	ProviderID         string `json:"provider_id"`
	FallbackProviderID string `json:"fallback_provider_id,omitempty"`
}

type SaveRouteRequest struct {
	ProviderID         string `json:"provider_id"`
	FallbackProviderID string `json:"fallback_provider_id"`
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
	if err := TestChat(ctx, cfg, apiKeyFor(cfg.ProviderType), req.Prompt); err != nil {
		return TestResponse{
			SchemaVersion: "provider.test.v1",
			ProviderID:    cfg.ProviderID,
			OK:            false,
			Message:       err.Error(),
			Endpoint:      ChatEndpoint(cfg),
			Model:         cfg.ChatModel,
		}
	}
	return TestResponse{
		SchemaVersion: "provider.test.v1",
		ProviderID:    cfg.ProviderID,
		OK:            true,
		Message:       "chat completion request succeeded",
		Endpoint:      ChatEndpoint(cfg),
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

func TestChat(ctx context.Context, cfg Config, apiKey string, prompt string) (err error) {
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
	endpoint := ChatEndpoint(cfg)
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
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		if readErr != nil {
			return readErr
		}
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

func ChatEndpoint(cfg Config) string {
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

type KeyCipher struct {
	secret string
}

func NewKeyCipher(secret string) KeyCipher {
	return KeyCipher{secret: strings.TrimSpace(secret)}
}

func (c KeyCipher) Configured() bool {
	return c.secret != ""
}

func (c KeyCipher) Encrypt(plain string) (string, error) {
	if !c.Configured() {
		return "", errors.New("PROVIDER_KEY_ENCRYPTION_SECRET is required to store API keys in database")
	}
	key := sha256.Sum256([]byte(c.secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (c KeyCipher) Decrypt(encoded string) (string, error) {
	if strings.TrimSpace(encoded) == "" {
		return "", nil
	}
	if !c.Configured() {
		return "", errors.New("PROVIDER_KEY_ENCRYPTION_SECRET is required to read database API keys")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(c.secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted API key")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
