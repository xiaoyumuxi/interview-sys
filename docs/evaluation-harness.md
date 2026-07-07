# Evaluation Harness

Evaluation Harness 是 Go Core API 的质量回归入口，用来保存可复用样例、调用 Python Runtime 执行任务，并把输出、断言、分数、错误和 agent trace 记录成 PostgreSQL 事实。

这些接口只允许 root 调用。

## API

| Endpoint | Purpose |
|---|---|
| `GET /api/evaluation/cases?suite=&task_type=&limit=` | 列出 active case |
| `POST /api/evaluation/cases` | 创建或更新 case |
| `GET /api/evaluation/cases/{case_id}` | 获取单个 case |
| `POST /api/evaluation/cases/{case_id}/run` | 执行 case 并记录 run |
| `GET /api/evaluation/runs?case_id=&task_type=&limit=` | 列出 run 记录 |

`POST /run` body 可为空；传 `{"dry_run": true}` 时会透传给 Python Runtime，适合接口检验和不调用真实模型的本地回归。

## Case Format

```json
{
  "case_id": "eval_question_generation_001",
  "suite": "runtime-smoke",
  "task_type": "question_generation",
  "skill_id": "java-backend",
  "input": {
    "user_input": "Generate one Redis failure recovery interview question.",
    "memory_query": "Redis failure recovery"
  },
  "expected": {
    "required_fields": ["question"],
    "contains": {
      "question": "Redis"
    },
    "output_schema": {
      "type": "object",
      "required": ["question"]
    }
  },
  "tags": ["runtime", "redis"],
  "status": "active"
}
```

`input.user_input` 会作为 Runtime task 的用户输入；未提供时 Go 会把整个 `input` JSON 压缩成用户输入。`input.memory_query` 用于 Context Engine 的 memory admission；未提供时复用 `user_input`。

## Assertions

第一版断言保持可配置但简单：

| Field | Behavior |
|---|---|
| `expected.required_fields` | 检查输出字段存在且非空，支持 `summary.overall` 这样的点路径 |
| `expected.contains` | 检查输出字段的字符串形式包含期望文本 |
| `expected.equals` | 检查输出字段等于期望值；类型不同但字符串形式一致也视为通过 |

如果未配置任何断言，run 会生成一个 `runtime_ok` 默认断言，只验证 Runtime 调用链路成功。

## Run Recording

每次运行都会写入 `evaluation_runs`：

- `passed`：所有断言通过。
- `failed`：Runtime 调用成功，但至少一个断言失败。
- `error`：case 存在，但 context preview、Provider resolution 或 Runtime 调用失败。

Runtime 成功时 Go 会同步写入 `agent_traces`，并把 `trace_id` 关联到 run。失败 run 也会保留 `error_text` 和 `duration_ms`，便于后续分析回归失败原因。
