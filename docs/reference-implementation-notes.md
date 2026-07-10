# 参考项目吸收记录

## 已吸收

### interview-guide

- Provider 配置要支持数据库和本地配置两种来源。
- Provider 更新/读取需要读写锁保护，避免运行时配置修改和请求读取互相踩踏。
- 结构化输出需要强制 JSON 指令、解析失败修复和重试。
- Prompt 安全边界要明确声明用户数据不是指令。

当前落地：

- Go Provider 支持 DeepSeek / OpenAI-compatible 配置。
- Provider 配置在启动时同步到 `provider_configs`。
- Provider CRUD、连通性测试、AES-GCM 密钥保存和 task route 切换已经落在 Go API。
- Python Runtime 包含结构化 JSON 解析和 Prompt data boundary。
- Evaluation Harness 已提供 root-only case/run API、dry-run、可配置断言和 agent trace 关联。

### AI-Meeting

- AI single-flight key 要按业务 stage、session、answer hash 或 business key 设计，避免重复 AI 调用。
- 面试状态机要区分会话状态和题目流转状态。
- Agent 场景绑定要让业务场景和具体 Agent 配置解耦。

当前落地：

- `agent_traces` 已记录 Go 调 Python Runtime 的输入、上下文和输出。
- Go Core 已落地 Redis single-flight，并把 session / flow / turn 三层状态机放在 Go 内集中校验。
- Answer 提交已经异步化，API 返回 `202 Accepted`，独立 `cmd/worker` 通过 Redis Stream 消费并推进 turn。
- Final report generation 已落地：Go 聚合 session/turn 确定性事实并持久化 `interview_reports`，Python Runtime `summary` task 只生成结构化报告内容。

### TechSpar

- Python 侧适合承载 LangGraph/Agent flow、Provider adapter、记忆候选提取。
- 记忆系统要区分知识轴和表现轴，behavior signal 要有限 namespace。
- Profile 更新不能直接全自动覆盖，应该走候选和审核。

当前落地：

- Python Runtime 独立服务已建立。
- Runtime 已支持 `memory_extraction` 任务，以及 memory candidates、review、edit、approve/reject、profile、search 和 due reviews API。
- Go `/api/memory/*` 已作为统一入口编排 Python memory API，负责鉴权、用户隔离、写操作 trace/audit 和错误标准化。
- Go Context Engine 已接入 approved memory admission，只允许审核通过的 memory 进入 Prompt，并保留 `memory_admission` 解释。

### AI-Meeting-Frontend

- 面试房间需要体现会议感、会话恢复、问答回放和报告阅读，而不是只堆 API 表单。
- ASR/TTS、摄像头和共享题面要和后端状态机解耦，先作为交互状态层落地，再接真实设备与同步接口。

当前落地：

- `frontend` 已建立 Vanilla TypeScript + Vite + Monaco 工作台，支持登录、中英文切换、工作台概览、会议式面试房间、代码题、memory review、settings 和 evaluation harness。
- 面试房间已有主舞台、候选人/Runtime 小窗、底部控制条、Companion 面板、trace/report 操作和本地 notes 状态。
- 真实音视频、ASR/TTS、共享题面同步和后端 notes API 仍作为后续产品化增强。

### aural-oss

- 代码题、白板、候选人邀请、反作弊和 SaaS 组织能力只作为远期参考。
- 当前项目先保留个人训练闭环，不进入招聘 SaaS 产品形态。

当前落地：

- CodeTop100 / 后端工程题库 schema、seed、查询 API、异步提交和 judge worker 已落地。
- 代码题 IDE 已在前端接入 Monaco，支持语言草稿、OJ 题面规格块、verdict 摘要，以及前端局部符号/快捷片段 + Go completion profile 的轻量联想补全。
- 招聘 SaaS、候选人邀请、反作弊、白板仍未进入当前阶段。

## 当前 Go 基础后端完成情况

阶段计划和下一批任务以 [roadmap.md](./roadmap.md) 为准。这里仅保留参考项目已经吸收进当前实现的记录。

已完成：

- Gin API 基础。
- Auth/User：JWT access token + JWT refresh token，密码 bcrypt 哈希，refresh token 只保存哈希。
- PostgreSQL 连接。
- Provider 配置启动同步入库。
- Skill Pack 扫描、热加载、创建、lint。
- Skill Pack 和 references 启动同步入库。
- Context Preview。
- Retrieval Harness MVP。
- Python Runtime client。
- Go -> Python Runtime dry-run 调用。
- agent trace 落库。
- CodeTop100 / 后端工程题库基础 schema 和查询 API。
- Coding completion profile API：`POST /api/coding/completions`，包含后端数据化标准库 catalog。
- Docker Compose 中间件固定版本和初始化脚本。
- Provider 配置的 CRUD API 和加密存储。
- Go Interview Runtime session / flow / turn 状态机。
- Go 侧 answer idempotency 和 Redis single-flight。
- PostgreSQL local message outbox, row-level claim, and short TTL Redis coordination without persisted lock fields.
- Redis Stream interview event queue。
- 独立 `cmd/worker` Redis Stream consumer group worker for answer evaluation。
- Redis Stream pending reclaim and dead-letter / poison message fallback。
- Dead-letter analyzer consumer and PostgreSQL `dead_letter_events` for external analysis.
- PostgreSQL runtime snapshot。
- Worker summary 运维接口。
- Python Runtime memory candidates、review、profile、search 和 due reviews API。
- Go API 到 Python Runtime memory API 的代理/编排。
- Final report generation。
- 代码执行 judge worker：Docker、warm Docker、native_trusted 和 disabled evaluator 模式。
- Coding judge summary 运维接口。
- Evaluation Harness MVP。
- 前端工作台 MVP。

未完成：

- 高级 RAG / KG / Search adapter 和多索引检索增强。
- LangGraph / Agent flow。
- 真实音视频、ASR/TTS、共享题面同步和后端 notes API。
- 函数签名式代码题适配、长驻 runner 服务、编译缓存和更细资源统计。
- 产品级报告页、队列式 memory review、evaluation 批量回归汇总和前端 smoke 测试。
