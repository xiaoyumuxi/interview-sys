INSERT INTO app_users (user_id, display_name)
VALUES ('local-user', 'Local User')
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO provider_configs (
    provider_id,
    provider_type,
    base_url,
    chat_endpoint_path,
    api_key_ref,
    chat_model,
    supports_streaming,
    supports_json,
    enabled
)
VALUES
    (
        'deepseek-default',
        'deepseek',
        'https://api.deepseek.com',
        '/chat/completions',
        'DEEPSEEK_API_KEY',
        'deepseek-v4-flash',
        TRUE,
        TRUE,
        TRUE
    ),
    (
        'openai-compatible-default',
        'openai_compatible',
        'https://api.openai.com/v1',
        '/chat/completions',
        'OPENAI_COMPATIBLE_API_KEY',
        NULL,
        TRUE,
        TRUE,
        FALSE
    ),
    (
        'embedding-default',
        'embedding',
        '',
        NULL,
        'EMBEDDING_API_KEY',
        NULL,
        FALSE,
        TRUE,
        FALSE
    )
ON CONFLICT (provider_id) DO UPDATE SET
    provider_type = EXCLUDED.provider_type,
    base_url = EXCLUDED.base_url,
    chat_endpoint_path = EXCLUDED.chat_endpoint_path,
    api_key_ref = EXCLUDED.api_key_ref,
    chat_model = EXCLUDED.chat_model,
    supports_streaming = EXCLUDED.supports_streaming,
    supports_json = EXCLUDED.supports_json,
    updated_at = now();

INSERT INTO provider_task_routes (task_type, provider_id, fallback_provider_id)
VALUES
    ('question_generation', 'deepseek-default', 'openai-compatible-default'),
    ('answer_evaluation', 'deepseek-default', 'openai-compatible-default'),
    ('follow_up_decision', 'deepseek-default', 'openai-compatible-default'),
    ('summary', 'deepseek-default', 'openai-compatible-default'),
    ('memory_extraction', 'deepseek-default', 'openai-compatible-default')
ON CONFLICT (task_type) DO UPDATE SET
    provider_id = EXCLUDED.provider_id,
    fallback_provider_id = EXCLUDED.fallback_provider_id,
    updated_at = now();

INSERT INTO code_question_sets (set_id, display_name, description, source, source_url, question_type)
VALUES
    (
        'codetop100-algorithm',
        'CodeTop100 高频算法题',
        '按 CodeTop 高频面试题整理的算法与数据结构题库。',
        'CodeTop100',
        'https://codetop.cc',
        'algorithm'
    ),
    (
        'backend-coding-engineering',
        '后端工程编程题',
        '面向后端岗位的并发、缓存、限流、LRU、SQL 和 API 设计编程题。',
        'local',
        '',
        'backend_engineering'
    )
ON CONFLICT (set_id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    source = EXCLUDED.source,
    source_url = EXCLUDED.source_url,
    question_type = EXCLUDED.question_type,
    updated_at = now();

DELETE FROM code_question_sets
WHERE set_id = 'backend-coding-basic'
  AND NOT EXISTS (
      SELECT 1 FROM code_questions WHERE code_questions.set_id = code_question_sets.set_id
  );

INSERT INTO code_questions (
    question_id,
    set_id,
    title,
    difficulty,
    source,
    source_url,
    question_type,
    frequency_rank,
    company_tags,
    topic_tags,
    prompt,
    input_format,
    output_format,
    constraints_text,
    reference_solution,
    explanation,
    status
)
VALUES (
    'two-sum',
    'codetop100-algorithm',
    'Two Sum',
    'easy',
    'CodeTop100',
    'https://codetop.cc',
    'algorithm',
    1,
    ARRAY[]::TEXT[],
    ARRAY['array', 'hash-table', 'algorithm-basic'],
    'Given an integer array nums and an integer target, return indices of the two numbers such that they add up to target. Assume exactly one solution and do not use the same element twice.',
    'nums as a JSON array and target as an integer.',
    'A JSON array with two indices.',
    '2 <= nums.length <= 10000.',
    'Use a hash map from value to index. For each value, check whether target - value appeared before.',
    'This checks whether the candidate can trade O(n^2) brute force for O(n) hash lookup.',
    'published'
)
ON CONFLICT (question_id) DO UPDATE SET
    set_id = EXCLUDED.set_id,
    title = EXCLUDED.title,
    difficulty = EXCLUDED.difficulty,
    source = EXCLUDED.source,
    source_url = EXCLUDED.source_url,
    question_type = EXCLUDED.question_type,
    frequency_rank = EXCLUDED.frequency_rank,
    company_tags = EXCLUDED.company_tags,
    topic_tags = EXCLUDED.topic_tags,
    prompt = EXCLUDED.prompt,
    input_format = EXCLUDED.input_format,
    output_format = EXCLUDED.output_format,
    constraints_text = EXCLUDED.constraints_text,
    reference_solution = EXCLUDED.reference_solution,
    explanation = EXCLUDED.explanation,
    status = EXCLUDED.status,
    updated_at = now();

INSERT INTO code_question_test_cases (test_case_id, question_id, case_type, input_text, expected_output, weight)
VALUES
    ('two-sum-sample-1', 'two-sum', 'sample', '{"nums":[2,7,11,15],"target":9}', '[0,1]', 1),
    ('two-sum-hidden-1', 'two-sum', 'hidden', '{"nums":[3,2,4],"target":6}', '[1,2]', 2)
ON CONFLICT (test_case_id) DO NOTHING;
