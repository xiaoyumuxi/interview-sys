# Go / Python 边界

## 原则

Go Core API 负责确定性业务，Python AI Runtime 负责非确定性 AI 推理。

如果一个动作会改变业务状态、写数据库主事实、影响幂等、影响审计，默认放在 Go。
如果一个动作是模型调用、Prompt 组装、结构化输出修复、Agent flow 或 RAG 计算，默认放在 Python。

## Go Core API

Go 负责：

- HTTP API。
- Auth / User。
- Provider 配置、任务路由和密钥引用。
- Skill Registry、Skill 热加载、Skill lint。
- Context Preview 和 Context Trace。
- Code question bank 管理和查询。
- Interview Runtime 状态机。
- Redis idempotency / single-flight。
- PostgreSQL 持久化和 snapshot restore。
- Agent trace / audit。
- 调用 Python Runtime，并决定什么时候可以推进状态。

Go 不负责：

- Prompt 细节迭代。
- LLM 结构化输出修复。
- LangGraph / Agent flow 内部编排。
- RAG 复杂检索和压缩算法。
- 直接执行用户代码。

## Python AI Runtime

Python 负责：

- LLM Adapter 执行。
- Prompt 安全边界。
- 结构化 JSON 输出解析/修复。
- Question Generation。
- Answer Evaluation。
- Follow-up Decision。
- Final Summary。
- Memory Candidate Extraction。
- Memory review、profile projection、人工审核。
- Review scheduler / SM-2 复习计划。
- Context Retrieval Harness / RAG / KG / Search Adapter。
- 后续 LangGraph Agent Flow。

Python 不允许：

- 直接推进 interview session 状态。
- 做 Redis single-flight 或业务幂等。
- 绕过 Go 写业务主表。
- 决定 Provider task routing。

## Provider 边界

Provider 配置和任务路由归 Go。

流程：

1. Go 从 `provider_task_routes` 和 `provider_configs` 解析任务使用的 Provider。
2. Go 根据 `api_key_source` 解析密钥：优先使用数据库加密密钥，`api_key_ref` 只作为 `.env` fallback/bootstrap。
3. Go 调用 Python Runtime 时传入本次 Provider 配置。
4. Python 只使用本次请求里的 Provider 执行模型调用。
5. Python trace 不记录 API key。

Provider/model 的创建、更新、删除、连通性测试和 task route 切换都走 Go API 和数据库。启动 seed 只补缺失默认值，不覆盖运行时配置。

Python 可以保留 env fallback 仅用于本地调试；生产链路以 Go 传入 Provider 为准。

## Trace 边界

- Go 写 `agent_traces`，记录输入、ContextItem、Runtime 输出和 trace id。
- Python 返回 runtime trace，但不直接写 Go 主库。
- 后续如果 Python 需要写内部调试日志，只能写 runtime-local debug，不作为业务事实来源。

## 当前状态

已完成：

- Go Provider/Skill 入库。
- Go Context Preview。
- Go 调 Python Runtime。
- Go 写 `agent_traces`。
- Python Runtime task endpoint。
- Python Prompt boundary 和结构化 JSON 解析。
- Python Runtime memory candidate/review/profile API。
- Provider CRUD、密钥加密存储和 task route 切换 API。
- Go Interview Runtime session / flow / turn 状态机。
- Go Redis single-flight / answer idempotency。
- PostgreSQL local message outbox for Redis Stream dispatch.
- PostgreSQL outbox row claim and turn state claim; Redis lock only for short TTL coordination, not persisted business state.
- Redis Stream interview event queue。
- Redis Stream consumer group worker for answer evaluation。
- PostgreSQL runtime snapshot。

未完成：

- Go API 到 Python Runtime memory API 的代理/编排。
- Retrieval Harness。
- Final report generation。
