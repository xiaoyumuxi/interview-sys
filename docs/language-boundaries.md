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
- Retrieval Harness MVP：组合 Skill reference、recent history、approved memory 和 summary，并返回 evidence、score、reason、source 与 warning。
- Memory API 对外编排、用户隔离和 trace/audit。
- Memory context admission：只允许 approved memory 以 `memory_context` 形式进入 Prompt，并保留 `memory_admission` 解释。
- Code question bank 管理、查询、OJ 题面字段、提交和 completion profile，包括后端数据化标准库 catalog。
- Coding judge worker 的 queued/running/terminal 状态推进、sandbox/native runner 选择和 `code_evaluation_traces`。
- Evaluation Harness 的 case/run 存储、dry-run、断言评分和 trace 关联。
- Interview Runtime 状态机。
- Redis idempotency / single-flight。
- PostgreSQL 持久化和 snapshot restore。
- Final report 的状态、持久化和确定性事实聚合。
- Agent trace / audit。
- 调用 Python Runtime，并决定什么时候可以推进状态。

Go 不负责：

- Prompt 细节迭代。
- LLM 结构化输出修复。
- LangGraph / Agent flow 内部编排。
- RAG 复杂检索、压缩和 LangGraph/Agent 编排算法。
- 在 API 请求线程中直接执行用户代码；用户代码只应通过受控 judge runner 路径处理。

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
- Memory search 和 memory scoring 解释。
- 后续高级 RAG / KG / Search Adapter。
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
- Go 对 `/api/memory/*` 写操作同样写 `agent_traces`，记录用户触发的 memory create/review/edit 编排事实。
- Python 返回 runtime trace，但不直接写 Go 主库。
- 后续如果 Python 需要写内部调试日志，只能写 runtime-local debug，不作为业务事实来源。

## 当前状态

已完成：

- Go Provider/Skill 入库。
- Go Auth/User JWT 双 Token。
- Go Context Preview。
- Go 调 Python Runtime。
- Go 写 `agent_traces`。
- Python Runtime task endpoint。
- Python Prompt boundary 和结构化 JSON 解析。
- Python Runtime memory candidate/review/profile API。
- Go `/api/memory/*` 编排 Python memory API。
- Go Context Engine 基于 Python memory search 结果执行 approved memory 准入和 token budget 装配。
- Provider CRUD、密钥加密存储和 task route 切换 API。
- Go Interview Runtime session / flow / turn 状态机。
- Go Redis single-flight / answer idempotency。
- PostgreSQL local message outbox for Redis Stream dispatch.
- PostgreSQL outbox row claim and turn state claim; Redis lock only for short TTL coordination, not persisted business state.
- Redis Stream interview event queue。
- Redis Stream consumer group worker for answer evaluation。
- PostgreSQL runtime snapshot。
- Dead-letter analyzer consumer 和 PostgreSQL `dead_letter_events`。
- Worker summary 运维接口。
- Final report generation：Go 聚合确定性事实并持久化 `interview_reports`，Python `summary` task 只生成结构化文本。
- Retrieval Harness MVP。
- Coding judge worker MVP：Docker、warm Docker、native_trusted 和 disabled evaluator 模式。
- Coding completion profile API 和后端数据化标准库 catalog。
- Evaluation Harness MVP。

未完成：

- 高级 RAG / KG / Search adapter。
- LangGraph / Agent flow。
- 真实音视频、ASR/TTS 和共享题面同步。
- 函数签名式代码题适配、长驻 runner 服务和更细的资源统计。
