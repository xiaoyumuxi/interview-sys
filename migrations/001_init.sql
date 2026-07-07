CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS app_users (
    user_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    email TEXT,
    password_hash TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('root', 'user')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE app_users ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE app_users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE app_users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';
ALTER TABLE app_users ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE app_users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_app_users_email ON app_users (lower(email)) WHERE email IS NOT NULL AND email <> '';

CREATE TABLE IF NOT EXISTS auth_refresh_tokens (
    session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES app_users(user_id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    user_agent TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_auth_refresh_tokens_user ON auth_refresh_tokens (user_id, revoked, expires_at);

CREATE TABLE IF NOT EXISTS provider_configs (
    provider_id TEXT PRIMARY KEY,
    provider_type TEXT NOT NULL,
    base_url TEXT NOT NULL,
    chat_endpoint_path TEXT,
    api_key_ref TEXT,
    api_key_source TEXT NOT NULL DEFAULT 'env_ref' CHECK (api_key_source IN ('env_ref', 'db_encrypted', 'none')),
    api_key_ciphertext TEXT,
    chat_model TEXT,
    embedding_model TEXT,
    supports_streaming BOOLEAN NOT NULL DEFAULT FALSE,
    supports_json BOOLEAN NOT NULL DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE provider_configs ADD COLUMN IF NOT EXISTS chat_endpoint_path TEXT;
ALTER TABLE provider_configs ADD COLUMN IF NOT EXISTS api_key_source TEXT NOT NULL DEFAULT 'env_ref';
ALTER TABLE provider_configs ADD COLUMN IF NOT EXISTS api_key_ciphertext TEXT;

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
    source TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    question_type TEXT NOT NULL DEFAULT 'algorithm',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE code_question_sets ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';
ALTER TABLE code_question_sets ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE code_question_sets ADD COLUMN IF NOT EXISTS question_type TEXT NOT NULL DEFAULT 'algorithm';

CREATE TABLE IF NOT EXISTS code_questions (
    question_id TEXT PRIMARY KEY,
    set_id TEXT REFERENCES code_question_sets(set_id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    difficulty TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    source TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    question_type TEXT NOT NULL DEFAULT 'algorithm',
    frequency_rank INTEGER,
    company_tags TEXT[] NOT NULL DEFAULT '{}',
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

ALTER TABLE code_questions ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';
ALTER TABLE code_questions ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE code_questions ADD COLUMN IF NOT EXISTS question_type TEXT NOT NULL DEFAULT 'algorithm';
ALTER TABLE code_questions ADD COLUMN IF NOT EXISTS frequency_rank INTEGER;
ALTER TABLE code_questions ADD COLUMN IF NOT EXISTS company_tags TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_code_questions_tags ON code_questions USING GIN (topic_tags);
CREATE INDEX IF NOT EXISTS idx_code_questions_company_tags ON code_questions USING GIN (company_tags);
CREATE INDEX IF NOT EXISTS idx_code_questions_status ON code_questions (status, difficulty);
CREATE INDEX IF NOT EXISTS idx_code_questions_source_type_rank ON code_questions (source, question_type, frequency_rank);

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

CREATE TABLE IF NOT EXISTS evaluation_cases (
    case_id TEXT PRIMARY KEY,
    suite TEXT NOT NULL DEFAULT 'default',
    task_type TEXT NOT NULL,
    skill_id TEXT,
    input JSONB NOT NULL DEFAULT '{}'::jsonb,
    expected JSONB NOT NULL DEFAULT '{}'::jsonb,
    tags TEXT[] NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_evaluation_cases_suite_status ON evaluation_cases (suite, status);
CREATE INDEX IF NOT EXISTS idx_evaluation_cases_task_type ON evaluation_cases (task_type, status);

CREATE TABLE IF NOT EXISTS evaluation_runs (
    run_id TEXT PRIMARY KEY,
    case_id TEXT NOT NULL REFERENCES evaluation_cases(case_id),
    task_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('passed', 'failed', 'error')),
    score NUMERIC(6,2) NOT NULL DEFAULT 0,
    input JSONB NOT NULL DEFAULT '{}'::jsonb,
    output JSONB NOT NULL DEFAULT '{}'::jsonb,
    assertions JSONB NOT NULL DEFAULT '[]'::jsonb,
    trace_id TEXT REFERENCES agent_traces(trace_id) ON DELETE SET NULL,
    error_text TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_evaluation_runs_case_created ON evaluation_runs (case_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_evaluation_runs_task_status ON evaluation_runs (task_type, status, created_at DESC);

CREATE TABLE IF NOT EXISTS interview_sessions (
    session_id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES app_users(user_id) ON DELETE SET NULL,
    skill_id TEXT NOT NULL,
    session_status TEXT NOT NULL DEFAULT 'READY' CHECK (session_status IN ('DRAFT', 'READY', 'IN_PROGRESS', 'FINISHED', 'ABANDONED', 'FAILED')),
    flow_status TEXT NOT NULL DEFAULT 'INIT' CHECK (flow_status IN ('INIT', 'ASKING', 'EVALUATING', 'FOLLOW_UP', 'COMPLETED')),
    phase TEXT NOT NULL DEFAULT 'technical',
    current_question_id TEXT REFERENCES code_questions(question_id) ON DELETE SET NULL,
    current_question_number INTEGER NOT NULL DEFAULT 0,
    answer_round INTEGER NOT NULL DEFAULT 0,
    follow_up_count INTEGER NOT NULL DEFAULT 0,
    max_follow_ups INTEGER NOT NULL DEFAULT 1,
    total_score NUMERIC(6,2) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_interview_sessions_user_status ON interview_sessions (user_id, session_status, updated_at DESC);

CREATE TABLE IF NOT EXISTS interview_turns (
    turn_id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES interview_sessions(session_id) ON DELETE CASCADE,
    question_id TEXT REFERENCES code_questions(question_id) ON DELETE SET NULL,
    question_number INTEGER NOT NULL,
    answer_round INTEGER NOT NULL,
    request_id TEXT NOT NULL,
    answer_hash TEXT NOT NULL,
    user_answer TEXT NOT NULL,
    turn_status TEXT NOT NULL DEFAULT 'queued' CHECK (turn_status IN ('queued', 'running', 'completed', 'failed')),
    processing_attempts INTEGER NOT NULL DEFAULT 0,
    evaluation JSONB NOT NULL DEFAULT '{}'::jsonb,
    follow_up_needed BOOLEAN NOT NULL DEFAULT FALSE,
    follow_up_question TEXT NOT NULL DEFAULT '',
    score NUMERIC(6,2) NOT NULL DEFAULT 0,
    trace_id TEXT REFERENCES agent_traces(trace_id) ON DELETE SET NULL,
    response JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (session_id, request_id),
    UNIQUE (session_id, question_number, answer_round, answer_hash)
);

ALTER TABLE interview_turns ADD COLUMN IF NOT EXISTS turn_status TEXT NOT NULL DEFAULT 'completed';
ALTER TABLE interview_turns ADD COLUMN IF NOT EXISTS error_text TEXT NOT NULL DEFAULT '';
ALTER TABLE interview_turns ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE interview_turns ADD COLUMN IF NOT EXISTS processing_attempts INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_interview_turns_session_created ON interview_turns (session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_interview_turns_status ON interview_turns (turn_status, updated_at);

CREATE TABLE IF NOT EXISTS interview_runtime_snapshots (
    session_id TEXT PRIMARY KEY REFERENCES interview_sessions(session_id) ON DELETE CASCADE,
    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL DEFAULT 'postgres',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS interview_reports (
    report_id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL UNIQUE REFERENCES interview_sessions(session_id) ON DELETE CASCADE,
    user_id TEXT REFERENCES app_users(user_id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    content JSONB NOT NULL DEFAULT '{}'::jsonb,
    runtime_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    trace_id TEXT REFERENCES agent_traces(trace_id) ON DELETE SET NULL,
    error_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_interview_reports_user_status ON interview_reports (user_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS async_messages (
    message_id TEXT PRIMARY KEY,
    stream_name TEXT NOT NULL,
    event_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL DEFAULT '',
    aggregate_id TEXT NOT NULL DEFAULT '',
    dedup_key TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'dispatching', 'dispatched', 'failed', 'dead_letter')),
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 20,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    redis_message_id TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    dispatched_at TIMESTAMPTZ,
    UNIQUE (stream_name, dedup_key)
);

ALTER TABLE async_messages DROP COLUMN IF EXISTS locked_by;
ALTER TABLE async_messages DROP COLUMN IF EXISTS locked_until;
ALTER TABLE async_messages DROP CONSTRAINT IF EXISTS async_messages_status_check;
ALTER TABLE async_messages ADD CONSTRAINT async_messages_status_check CHECK (status IN ('pending', 'dispatching', 'dispatched', 'failed', 'dead_letter'));

CREATE INDEX IF NOT EXISTS idx_async_messages_dispatch ON async_messages (status, next_retry_at, created_at);
CREATE INDEX IF NOT EXISTS idx_async_messages_aggregate ON async_messages (aggregate_type, aggregate_id, created_at);

CREATE TABLE IF NOT EXISTS dead_letter_events (
    dead_letter_id TEXT PRIMARY KEY,
    source TEXT NOT NULL CHECK (source IN ('redis_stream', 'async_outbox')),
    source_stream TEXT NOT NULL DEFAULT '',
    source_message_id TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL DEFAULT '',
    aggregate_type TEXT NOT NULL DEFAULT '',
    aggregate_id TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    reason TEXT NOT NULL DEFAULT '',
    error_text TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'analyzing', 'resolved', 'ignored')),
    occurrence_count INTEGER NOT NULL DEFAULT 1,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source, source_stream, source_message_id)
);

CREATE INDEX IF NOT EXISTS idx_dead_letter_events_status ON dead_letter_events (status, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_dead_letter_events_aggregate ON dead_letter_events (aggregate_type, aggregate_id, last_seen_at DESC);
