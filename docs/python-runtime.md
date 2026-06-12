# Python AI Runtime

Python Runtime 负责非确定性 AI 推理：

- LLM Adapter。
- 结构化输出。
- Prompt 安全边界。
- Question Generation。
- Answer Evaluation。
- Follow-up Decision。
- Summary。
- Memory Candidate Extraction。
- Memory review。
- Profile Projection。
- Review scheduler。
- 后续 Retrieval Harness / LangGraph Flow。

Go Core API 仍负责状态机、幂等、single-flight、审计和 Provider 路由。Memory candidate、review、profile projection 和 review scheduler 放在 Python Runtime；Python 不直接推进面试状态，不决定 Provider task routing。

## API

```text
GET  /healthz
POST /api/runtime/tasks
GET  /api/runtime/memory/candidates
POST /api/runtime/memory/candidates
POST /api/runtime/memory/candidates/{candidate_id}/approve
POST /api/runtime/memory/candidates/{candidate_id}/reject
POST /api/runtime/memory/candidates/{candidate_id}/edit
GET  /api/runtime/memory/profile
GET  /api/runtime/memory/search
GET  /api/runtime/reviews/due
```

`POST /api/runtime/tasks` 支持：

```text
question_generation
answer_evaluation
follow_up_decision
summary
memory_extraction
```

## 启动

```bash
docker compose --profile runtime up -d python-runtime
```

本地开发：

```bash
cd python-runtime
uv sync
uv run uvicorn app.main:app --host 0.0.0.0 --port 8090
```

测试：

```bash
uv run python -m unittest discover -s tests -p 'test_*.py' -v
```

## 参考吸收

- TechSpar：LangGraph 状态机、phase advance、记忆提取和 behavior signals。
- interview-guide：结构化 JSON 输出、重试修复、Prompt 注入防御边界。
- AI-Meeting：Agent 场景绑定、AI single-flight 语义。single-flight 留在 Go Core。
