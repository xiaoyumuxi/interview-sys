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
	db *sql.DB
}

func Open(databaseURL string) (*Store, error) {
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
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) UpsertProviderConfigs(ctx context.Context, items []provider.Config) error {
	for _, item := range items {
		_, err := s.db.ExecContext(ctx, `
INSERT INTO provider_configs (
  provider_id, provider_type, base_url, chat_endpoint_path, chat_model,
  embedding_model, supports_streaming, supports_json, enabled, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
ON CONFLICT (provider_id) DO UPDATE SET
  provider_type = EXCLUDED.provider_type,
  base_url = EXCLUDED.base_url,
  chat_endpoint_path = EXCLUDED.chat_endpoint_path,
  chat_model = EXCLUDED.chat_model,
  embedding_model = EXCLUDED.embedding_model,
  supports_streaming = EXCLUDED.supports_streaming,
  supports_json = EXCLUDED.supports_json,
  enabled = EXCLUDED.enabled,
  updated_at = now()`,
			item.ProviderID,
			string(item.ProviderType),
			item.BaseURL,
			item.ChatEndpointPath,
			nullEmpty(item.ChatModel),
			nullEmpty(item.EmbeddingModel),
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
       COALESCE(p.api_key_ref,''), p.supports_json
FROM provider_task_routes r
JOIN provider_configs p ON p.provider_id = r.provider_id
WHERE r.task_type=$1`, taskType)

	var cfg airuntime.ProviderConfig
	var apiKeyRef string
	if err := row.Scan(&cfg.ProviderType, &cfg.BaseURL, &cfg.ChatEndpointPath, &cfg.Model, &apiKeyRef, &cfg.SupportsJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	cfg.APIKey = os.Getenv(apiKeyRef)
	return &cfg, nil
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
