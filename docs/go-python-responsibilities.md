# Go / Python Responsibilities

这份文档只说明当前项目里 Go 后端和 Python Runtime 分别负责什么，后续实现先按这里判断边界。

## Go Core API

Go 负责确定性业务事实、状态推进、幂等和对外 API。

Go 当前负责：

- HTTP API 和路由入口。
- Auth / User、JWT 双 Token、root 权限和用户隔离。
- Provider 配置、密钥来源、任务路由和模型切换。
- Skill Registry、Skill 扫描、热加载和 lint。
- Context Preview 的基础组装和调试接口。
- Retrieval Harness MVP：组合 Skill reference、recent history、summary 和 approved memory，并返回 evidence、score、reason、source 和 debug trace。
- Memory API 对外编排：鉴权、用户隔离、写操作 trace/audit 和错误标准化。
- Memory context admission：按 user、task_type、skill、query 和 token budget 将 approved memory 转为可解释的 `memory_context`。
- Code question bank 的 schema、seed、查询接口、OJ 题面字段和 coding completion profile，包括后端数据化标准库 catalog。
- Code submission、judge worker 状态推进、sandbox/native runner 选择、verdict 和 `code_evaluation_traces`。
- Evaluation Harness 的 case/run 存储、dry-run 调用、断言评分和 trace 关联。
- Interview session / flow / turn 状态机。
- Answer 提交幂等、turn claim 和状态落库。
- PostgreSQL local outbox。
- Redis Stream dispatch、consumer group worker、pending reclaim 和 dead-letter。
- Dead-letter analyzer consumer、错误数据标准化和对外查询 API。
- Redis single-flight 和短 TTL 协调锁。
- PostgreSQL runtime snapshot。
- Final report 的状态、持久化和确定性事实聚合。
- Agent trace / audit 落库。
- 调用 Python Runtime，并根据返回结果决定是否推进 Go 状态机。

Go 不负责：

- Prompt 细节。
- LLM 输出修复。
- Agent flow / LangGraph 编排。
- 高级 RAG / KG / Search adapter 的复杂检索与压缩。
- Memory candidate、review、profile projection、review scheduler 的主逻辑。
- 在 API 请求线程中直接执行用户代码；用户代码只应通过 Go 管理的受控 judge runner 路径处理。

## Python AI Runtime

Python 负责非确定性 AI 推理、Agent/RAG 计算和 memory 系统。

Python 当前负责：

- LLM Adapter 执行。
- Prompt 安全边界。
- 结构化 JSON 输出解析。
- Question generation。
- Answer evaluation。
- Follow-up decision。
- Summary。
- Memory candidate extraction。
- Memory candidate 存储、review、edit、approve/reject。
- Profile projection。
- Review scheduler / due review。
- Memory search 和 `memory_context_score` 解释。
  Go 使用该解释作为 memory 准入 trace，不在 Go 中重写 memory 排序算法。
- Summary task 生成 final report 所需的结构化文本内容；报告状态和持久化仍由 Go 控制。

Python 后续负责：

- 高级 RAG / KG / Search adapter。
- LangGraph / Agent flow。
- 更复杂的模型质量评估逻辑；Evaluation Harness 的 case/run、断言和回归记录仍由 Go 控制。

Python 不负责：

- 推进 interview session / flow / turn 状态。
- 做 Redis single-flight 或业务幂等。
- 决定 Provider task routing。
- 绕过 Go 改 Go 的业务主表。

## Boundary Rules

- 改变面试状态、请求幂等、worker 消费和审计事实：放 Go。
- 代码题提交、判题状态、补全画像和运行审计：放 Go。
- 模型调用、Prompt、结构化输出、memory、RAG 和 Agent 推理：放 Python。
- Go 通过 `/api/memory/*` 代理或编排 Python Runtime memory API，并在 Context Engine 里决定哪些 approved memory 可进入 Prompt；memory 的主逻辑、搜索和评分仍留在 Python。
- Python 可以保存 runtime-local memory 数据，但不能直接推进 Go Interview Runtime。
