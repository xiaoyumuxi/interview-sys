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
- Memory search 和 memory scoring 解释。
- 后续高级 RAG / KG / Search Adapter 和 LangGraph Flow。

Go Core API 仍负责状态机、幂等、single-flight、审计、Provider 路由、Retrieval Harness MVP、Evaluation Harness 记录/断言、Final report 持久化和 coding judge。Memory candidate、review、profile projection 和 review scheduler 放在 Python Runtime；Python 不直接推进面试状态，不决定 Provider task routing，不执行用户代码。

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

`summary` 用于生成 final report 所需的结构化文本内容。Go 负责聚合 session/turn 确定性事实、调用 Runtime、写入 `interview_reports` 并控制报告状态。

Retrieval 当前第一版在 Go Context Engine 内完成：组合 Skill reference、recent history、summary 和 approved memory，并保留 evidence/source/reason/debug trace。Python Runtime 当前提供 memory search 和 scoring 解释；复杂 RAG、KG、Search adapter 或 LangGraph flow 后续再放到 Python。

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
