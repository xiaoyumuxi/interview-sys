CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS app_users (
    user_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS provider_configs (
    provider_id TEXT PRIMARY KEY,
    provider_type TEXT NOT NULL,
    base_url TEXT NOT NULL,
    chat_endpoint_path TEXT,
    api_key_ref TEXT,
    chat_model TEXT,
    embedding_model TEXT,
    supports_streaming BOOLEAN NOT NULL DEFAULT FALSE,
    supports_json BOOLEAN NOT NULL DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE provider_configs ADD COLUMN IF NOT EXISTS chat_endpoint_path TEXT;

CREATE TABLE IF NOT EXISTS provider_task_routes (
    task_type TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL REFERENCES provider_configs(provider_id),
    fallback_provider_id TEXT REFERENCES provider_configs(provider_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS skill_packs (
    skill_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    meta JSONB NOT NULL DEFAULT '{}'::jsonb,
    instructions TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS skill_references (
    reference_id TEXT PRIMARY KEY,
    skill_id TEXT NOT NULL REFERENCES skill_packs(skill_id) ON DELETE CASCADE,
    source_path TEXT NOT NULL,
    content TEXT NOT NULL,
    content_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,
    embedding VECTOR(1536),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_references_tsv ON skill_references USING GIN (content_tsv);

CREATE TABLE IF NOT EXISTS code_question_sets (
    set_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS code_questions (
    question_id TEXT PRIMARY KEY,
    set_id TEXT REFERENCES code_question_sets(set_id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    difficulty TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    topic_tags TEXT[] NOT NULL DEFAULT '{}',
    prompt TEXT NOT NULL,
    input_format TEXT NOT NULL DEFAULT '',
    output_format TEXT NOT NULL DEFAULT '',
    constraints_text TEXT NOT NULL DEFAULT '',
    reference_solution TEXT NOT NULL DEFAULT '',
    explanation TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_code_questions_tags ON code_questions USING GIN (topic_tags);
CREATE INDEX IF NOT EXISTS idx_code_questions_status ON code_questions (status, difficulty);

CREATE TABLE IF NOT EXISTS code_question_test_cases (
    test_case_id TEXT PRIMARY KEY,
    question_id TEXT NOT NULL REFERENCES code_questions(question_id) ON DELETE CASCADE,
    case_type TEXT NOT NULL CHECK (case_type IN ('sample', 'hidden')),
    input_text TEXT NOT NULL,
    expected_output TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1 CHECK (weight > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS code_submissions (
    submission_id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES app_users(user_id) ON DELETE SET NULL,
    question_id TEXT NOT NULL REFERENCES code_questions(question_id),
    language TEXT NOT NULL,
    source_code TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'accepted', 'wrong_answer', 'runtime_error', 'time_limit_exceeded', 'compile_error', 'system_error')),
    score NUMERIC(5,2) NOT NULL DEFAULT 0,
    result JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS code_evaluation_traces (
    trace_id TEXT PRIMARY KEY,
    submission_id TEXT NOT NULL REFERENCES code_submissions(submission_id) ON DELETE CASCADE,
    judge_worker_id TEXT,
    test_results JSONB NOT NULL DEFAULT '[]'::jsonb,
    stdout_text TEXT NOT NULL DEFAULT '',
    stderr_text TEXT NOT NULL DEFAULT '',
    resource_usage JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS agent_traces (
    trace_id TEXT PRIMARY KEY,
    task_type TEXT NOT NULL,
    skill_id TEXT,
    input JSONB NOT NULL DEFAULT '{}'::jsonb,
    context_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    output JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
