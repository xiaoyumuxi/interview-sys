package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"ai-interview-platform/internal/provider"
	airuntime "ai-interview-platform/internal/runtime"
	"ai-interview-platform/internal/skill"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db        *sql.DB
	keyCipher provider.KeyCipher
}

func Open(databaseURL string, providerKeyEncryptionSecret string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, keyCipher: provider.NewKeyCipher(providerKeyEncryptionSecret)}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) SeedProviderConfigs(ctx context.Context, items []provider.Config) error {
	for _, item := range items {
		_, err := s.db.ExecContext(ctx, `
INSERT INTO provider_configs (
  provider_id, provider_type, base_url, chat_endpoint_path, chat_model,
  embedding_model, api_key_ref, api_key_source, supports_streaming, supports_json, enabled, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())
ON CONFLICT (provider_id) DO NOTHING`,
			item.ProviderID,
			string(item.ProviderType),
			item.BaseURL,
			item.ChatEndpointPath,
			nullEmpty(item.ChatModel),
			nullEmpty(item.EmbeddingModel),
			nullEmpty(item.APIKeyRef),
			string(item.APIKeySource),
			item.SupportsStreaming,
			item.SupportsJSON,
			item.Enabled,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListProviders(ctx context.Context) ([]provider.Config, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT provider_id, provider_type, base_url, COALESCE(chat_endpoint_path,''), COALESCE(api_key_ref,''),
       COALESCE(api_key_source,'env_ref'), COALESCE(api_key_ciphertext,''),
       COALESCE(chat_model,''), COALESCE(embedding_model,''), supports_streaming, supports_json, enabled
FROM provider_configs
ORDER BY provider_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []provider.Config
	for rows.Next() {
		item, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetProvider(ctx context.Context, providerID string) (provider.Config, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT provider_id, provider_type, base_url, COALESCE(chat_endpoint_path,''), COALESCE(api_key_ref,''),
       COALESCE(api_key_source,'env_ref'), COALESCE(api_key_ciphertext,''),
       COALESCE(chat_model,''), COALESCE(embedding_model,''), supports_streaming, supports_json, enabled
FROM provider_configs
WHERE provider_id=$1`, providerID)
	item, err := scanProvider(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return provider.Config{}, false, nil
		}
		return provider.Config{}, false, err
	}
	return item, true, nil
}

func (s *Store) SaveProvider(ctx context.Context, req provider.SaveRequest) (provider.Config, error) {
	if req.ProviderID == "" {
		return provider.Config{}, fmt.Errorf("provider_id is required")
	}
	if req.ProviderType == "" {
		return provider.Config{}, fmt.Errorf("provider_type is required")
	}
	if req.BaseURL == "" {
		return provider.Config{}, fmt.Errorf("base_url is required")
	}

	apiKeySource, apiKeyRef, encrypted, err := s.providerKeyState(ctx, req.ProviderID)
	if err != nil {
		return provider.Config{}, err
	}
	if apiKeySource == "" {
		apiKeySource = provider.KeySourceNone
	}
	if req.ClearAPIKey {
		apiKeySource = provider.KeySourceNone
		apiKeyRef = ""
		encrypted = nil
	} else if req.APIKey != "" {
		ciphertext, err := s.keyCipher.Encrypt(req.APIKey)
		if err != nil {
			return provider.Config{}, err
		}
		apiKeySource = provider.KeySourceDBEncrypted
		encrypted = ciphertext
	} else if req.APIKeyRef != "" {
		apiKeySource = provider.KeySourceEnvRef
		apiKeyRef = req.APIKeyRef
		encrypted = nil
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO provider_configs (
  provider_id, provider_type, base_url, chat_endpoint_path, api_key_ref, api_key_source,
  api_key_ciphertext, chat_model, embedding_model, supports_streaming, supports_json, enabled, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now())
ON CONFLICT (provider_id) DO UPDATE SET
  provider_type = EXCLUDED.provider_type,
  base_url = EXCLUDED.base_url,
  chat_endpoint_path = EXCLUDED.chat_endpoint_path,
  api_key_ref = EXCLUDED.api_key_ref,
  api_key_source = EXCLUDED.api_key_source,
  api_key_ciphertext = CASE
    WHEN EXCLUDED.api_key_source = 'db_encrypted' THEN EXCLUDED.api_key_ciphertext
    WHEN EXCLUDED.api_key_source = 'none' THEN NULL
    ELSE NULL
  END,
  chat_model = EXCLUDED.chat_model,
  embedding_model = EXCLUDED.embedding_model,
  supports_streaming = EXCLUDED.supports_streaming,
  supports_json = EXCLUDED.supports_json,
  enabled = EXCLUDED.enabled,
  updated_at = now()`,
		req.ProviderID,
		string(req.ProviderType),
		req.BaseURL,
		nullEmpty(req.ChatEndpointPath),
		nullEmpty(apiKeyRef),
		string(apiKeySource),
		encrypted,
		nullEmpty(req.ChatModel),
		nullEmpty(req.EmbeddingModel),
		req.SupportsStreaming,
		req.SupportsJSON,
		req.Enabled,
	)
	if err != nil {
		return provider.Config{}, err
	}
	item, ok, err := s.GetProvider(ctx, req.ProviderID)
	if err != nil {
		return provider.Config{}, err
	}
	if !ok {
		return provider.Config{}, fmt.Errorf("provider was not saved")
	}
	return item, nil
}

func (s *Store) providerKeyState(ctx context.Context, providerID string) (provider.KeySource, string, any, error) {
	var source, ref, ciphertext string
	err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(api_key_source,'env_ref'), COALESCE(api_key_ref,''), COALESCE(api_key_ciphertext,'')
FROM provider_configs
WHERE provider_id=$1`, providerID).Scan(&source, &ref, &ciphertext)
	if err != nil {
		if err == sql.ErrNoRows {
			return provider.KeySourceNone, "", nil, nil
		}
		return "", "", nil, err
	}
	var encrypted any
	if ciphertext != "" {
		encrypted = ciphertext
	}
	return provider.KeySource(source), ref, encrypted, nil
}

func (s *Store) DeleteProvider(ctx context.Context, providerID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM provider_configs WHERE provider_id=$1`, providerID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListProviderRoutes(ctx context.Context) ([]provider.Route, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT task_type, provider_id, COALESCE(fallback_provider_id,'')
FROM provider_task_routes
ORDER BY task_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var routes []provider.Route
	for rows.Next() {
		var route provider.Route
		if err := rows.Scan(&route.TaskType, &route.ProviderID, &route.FallbackProviderID); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (s *Store) SaveProviderRoute(ctx context.Context, taskType string, req provider.SaveRouteRequest) (provider.Route, error) {
	if taskType == "" {
		return provider.Route{}, fmt.Errorf("task_type is required")
	}
	if req.ProviderID == "" {
		return provider.Route{}, fmt.Errorf("provider_id is required")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO provider_task_routes (task_type, provider_id, fallback_provider_id, updated_at)
VALUES ($1,$2,$3,now())
ON CONFLICT (task_type) DO UPDATE SET
  provider_id = EXCLUDED.provider_id,
  fallback_provider_id = EXCLUDED.fallback_provider_id,
  updated_at = now()`,
		taskType,
		req.ProviderID,
		nullEmpty(req.FallbackProviderID),
	)
	if err != nil {
		return provider.Route{}, err
	}
	return provider.Route{TaskType: taskType, ProviderID: req.ProviderID, FallbackProviderID: req.FallbackProviderID}, nil
}

func (s *Store) ProviderTestConfig(ctx context.Context, providerID string) (provider.Config, string, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT provider_id, provider_type, base_url, COALESCE(chat_endpoint_path,''), COALESCE(api_key_ref,''),
       COALESCE(api_key_source,'env_ref'), COALESCE(api_key_ciphertext,''),
       COALESCE(chat_model,''), COALESCE(embedding_model,''), supports_streaming, supports_json, enabled
FROM provider_configs
WHERE provider_id=$1`, providerID)
	item, ciphertext, err := scanProviderWithCiphertext(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return provider.Config{}, "", false, nil
		}
		return provider.Config{}, "", false, err
	}
	key, err := s.resolveAPIKey(item.APIKeySource, item.APIKeyRef, ciphertext)
	if err != nil {
		return provider.Config{}, "", false, err
	}
	return item, key, true, nil
}

func (s *Store) SyncSkills(ctx context.Context, skills []skill.Skill) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, item := range skills {
		meta, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO skill_packs (skill_id, display_name, description, meta, instructions, updated_at)
VALUES ($1,$2,$3,$4,$5,now())
ON CONFLICT (skill_id) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  description = EXCLUDED.description,
  meta = EXCLUDED.meta,
  instructions = EXCLUDED.instructions,
  updated_at = now()`,
			item.ID, item.DisplayName, item.Description, meta, item.Instructions,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM skill_references WHERE skill_id=$1`, item.ID); err != nil {
			return err
		}
		for _, ref := range item.References {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO skill_references (reference_id, skill_id, source_path, content, updated_at)
VALUES ($1,$2,$3,$4,now())
ON CONFLICT (reference_id) DO UPDATE SET
  content = EXCLUDED.content,
  updated_at = now()`,
				ref.SourceID, item.ID, ref.SourceID, ref.Content,
			); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

type TraceRecord struct {
	TraceID      string
	TaskType     string
	SkillID      string
	Input        any
	ContextItems any
	Output       any
}

func (s *Store) InsertAgentTrace(ctx context.Context, record TraceRecord) error {
	input, err := json.Marshal(record.Input)
	if err != nil {
		return err
	}
	contextItems, err := json.Marshal(record.ContextItems)
	if err != nil {
		return err
	}
	output, err := json.Marshal(record.Output)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO agent_traces (trace_id, task_type, skill_id, input, context_items, output)
VALUES ($1,$2,$3,$4,$5,$6)`,
		record.TraceID, record.TaskType, nullEmpty(record.SkillID), input, contextItems, output,
	)
	return err
}

func (s *Store) RuntimeProviderForTask(ctx context.Context, taskType string) (*airuntime.ProviderConfig, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT p.provider_type, p.base_url, COALESCE(p.chat_endpoint_path,''), COALESCE(p.chat_model,''),
       COALESCE(p.api_key_ref,''), COALESCE(p.api_key_source,'env_ref'), COALESCE(p.api_key_ciphertext,''), p.supports_json
FROM provider_task_routes r
JOIN provider_configs p ON p.provider_id = r.provider_id
WHERE r.task_type=$1 AND p.enabled=true`, taskType)

	var cfg airuntime.ProviderConfig
	var apiKeyRef, apiKeySource, apiKeyCiphertext string
	if err := row.Scan(&cfg.ProviderType, &cfg.BaseURL, &cfg.ChatEndpointPath, &cfg.Model, &apiKeyRef, &apiKeySource, &apiKeyCiphertext, &cfg.SupportsJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	key, err := s.resolveAPIKey(provider.KeySource(apiKeySource), apiKeyRef, apiKeyCiphertext)
	if err != nil {
		return nil, err
	}
	cfg.APIKey = key
	return &cfg, nil
}

func (s *Store) resolveAPIKey(source provider.KeySource, ref string, ciphertext string) (string, error) {
	switch source {
	case provider.KeySourceDBEncrypted:
		return s.keyCipher.Decrypt(ciphertext)
	case provider.KeySourceNone:
		return "", nil
	default:
		return os.Getenv(ref), nil
	}
}

type providerScanner interface {
	Scan(dest ...any) error
}

func scanProvider(row providerScanner) (provider.Config, error) {
	item, _, err := scanProviderWithCiphertext(row)
	return item, err
}

func scanProviderWithCiphertext(row providerScanner) (provider.Config, string, error) {
	var item provider.Config
	var providerType, apiKeySource, ciphertext string
	if err := row.Scan(
		&item.ProviderID,
		&providerType,
		&item.BaseURL,
		&item.ChatEndpointPath,
		&item.APIKeyRef,
		&apiKeySource,
		&ciphertext,
		&item.ChatModel,
		&item.EmbeddingModel,
		&item.SupportsStreaming,
		&item.SupportsJSON,
		&item.Enabled,
	); err != nil {
		return provider.Config{}, "", err
	}
	item.ProviderType = provider.Type(providerType)
	item.APIKeySource = provider.KeySource(apiKeySource)
	item.APIKeyConfigured = ciphertext != "" || (item.APIKeyRef != "" && os.Getenv(item.APIKeyRef) != "")
	return item, ciphertext, nil
}

func nullEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func NewID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
