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
- Code question bank 的 schema、seed 和查询接口。
- Interview session / flow / turn 状态机。
- Answer 提交幂等、turn claim 和状态落库。
- PostgreSQL local outbox。
- Redis Stream dispatch、consumer group worker、pending reclaim 和 dead-letter。
- Redis single-flight 和短 TTL 协调锁。
- PostgreSQL runtime snapshot。
- Agent trace / audit 落库。
- 调用 Python Runtime，并根据返回结果决定是否推进 Go 状态机。

Go 不负责：

- Prompt 细节。
- LLM 输出修复。
- Agent flow / LangGraph 编排。
- RAG / Retrieval Harness 的复杂检索与压缩。
- Memory candidate、review、profile projection、review scheduler。
- 直接执行用户代码。

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

Python 后续负责：

- Retrieval Harness。
- RAG / KG / Search adapter。
- LangGraph / Agent flow。
- Final report 内容生成。
- Evaluation Harness 的质量样例评估。

Python 不负责：

- 推进 interview session / flow / turn 状态。
- 做 Redis single-flight 或业务幂等。
- 决定 Provider task routing。
- 绕过 Go 改 Go 的业务主表。

## Boundary Rules

- 改变面试状态、请求幂等、worker 消费和审计事实：放 Go。
- 模型调用、Prompt、结构化输出、memory、RAG 和 Agent 推理：放 Python。
- Go 可以代理或编排 Python Runtime API，但 memory 的主逻辑仍留在 Python。
- Python 可以保存 runtime-local memory 数据，但不能直接推进 Go Interview Runtime。
