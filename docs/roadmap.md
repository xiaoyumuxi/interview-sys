# 项目路线图

这份文档是当前唯一的阶段计划入口。历史设计草案已经合并到 README、AGENTS 和 docs 下的专题文档中，后续以这里记录当前状态和下一步任务。

## 当前定位

本项目是个人 AI 面试训练平台后端，当前重点是先完成可解释、可回放、可测试的后端闭环。Go Core API 负责确定性业务事实、状态机、幂等和审计；Python AI Runtime 负责模型调用、Prompt、结构化输出、memory 和后续 RAG/Agent 推理。

当前不做招聘 SaaS、组织协作、候选人邀请、反作弊和完整前端重写。这些能力只保留接口和设计余量。

## 已完成

- Go Core API、Gin 路由、健康检查和基础 HTTP API。
- Auth/User：JWT 双 Token、bcrypt 密码哈希、refresh token 哈希存储、默认 root。
- Docker Compose 中间件：PostgreSQL + pgvector、Redis、MinIO。
- 跨平台 bootstrap、init-db 和 middleware manifest 检查脚本。
- Provider 配置入库，支持 DeepSeek 和 OpenAI-compatible。
- Provider API key 支持 env fallback 和数据库 AES-GCM 加密保存，接口不回显密钥。
- Provider CRUD、连通性测试、模型切换和 task route 配置。
- Skill Pack 本地扫描、创建、热加载、lint 和提示词注入基础校验。
- Java 后端 Skill 覆盖通用后端、Java、网络、分布式、MySQL、Redis、Spring、系统设计、算法、代码题和项目经历。
- Context Preview 和 Agent Trace 基础链路。
- Python AI Runtime 基础服务，使用 `uv` 管理依赖。
- Python Runtime task endpoint，支持 prompt 安全边界和结构化 JSON 解析。
- Python Runtime memory candidates、review、edit、approve/reject、profile、search 和 due reviews API。
- Go API `/api/memory/*` 编排 Python memory API，统一处理鉴权、用户隔离、写操作 trace/audit 和错误标准化。
- Memory context admission rules：Go Context Engine 可按 user、task_type、skill、query 和 token budget 引入 approved memory，并返回 evidence/source/reason 级别的 `memory_admission` trace。
- CodeTop100 / 后端工程题库 schema、seed 和查询 API。
- Interview Runtime session / flow / turn 三层状态机。
- Answer 提交异步化：API 返回 `202 Accepted`，worker 消费 Redis Stream 后评估。
- PostgreSQL local outbox `async_messages`，支持 Redis Stream 补投、重试和 dedup。
- Redis single-flight、短 TTL Redis 协调、stale turn reclaim 和数据库幂等。
- PostgreSQL runtime snapshot，用于 Redis 丢失后的业务事实恢复。
- 独立 `cmd/worker` 进程，API 默认只负责入队和查询。
- Redis Stream pending reclaim、dead-letter / poison message 兜底。
- Dead-letter analyzer consumer 和 PostgreSQL `dead_letter_events` 统一分析表。
- Worker summary 运维接口：`GET /api/ops/workers/summary`。
- Final report generation：Go 聚合 session/turn 确定性事实并持久化 `interview_reports`，Python Runtime `summary` task 生成结构化报告内容。
- Retrieval Harness MVP：`POST /api/retrieval/search` 返回 Skill reference、summary、recent history、approved memory 的 evidence、score、reason、source 和 debug trace；vector 索引未建立时显式返回 warning。
- Coding judge worker MVP：Go 支持 queued submission claim、running/terminal 状态推进、`code_evaluation_traces` 写入和 `GET /api/ops/coding-judge/summary`；`docker` 每次创建临时禁网容器，`docker_warm` 复用按语言命名的 stopped container 并用 tmpfs 回到初始状态，二者都支持 Go、Java、Python、JavaScript、TypeScript、C++ 完整程序和可配置镜像；`native_trusted` 可直接调用本机工具链加快本地可信开发，默认 disabled evaluator 不执行用户代码。

## 下一批任务

优先级按“能形成产品闭环”和“能降低后续返工风险”排序。

1. Coding judge runner 增强。
   - 当前 Docker sandbox MVP 支持 Go、Java、Python、JavaScript、TypeScript、C++ 完整程序。
   - 后续扩展统一编译缓存、镜像预拉取检查、更细的资源统计、长驻 runner 服务和函数签名式题目适配。

2. Evaluation Harness。
   - 建立出题、评分、追问、总结、RAG 命中和 memory 提取的样例集。
   - 记录质量、成本、延迟和回归结果。
   - 先服务后端稳定性，不急于做复杂评测平台。

3. 前端接入准备。
   - 明确异步 trace 轮询、session 状态展示、报告页和 memory review 的接口契约。
   - 复用现有后端 schema version，避免前端直接绑定内部实现。

## 参考来源使用原则

详细索引见 [reference-projects.md](./reference-projects.md)。后续实现前按模块回看对应参考项目，但不要直接复制外部实现。

- AI-Meeting：面试状态机、追问推进、answer pipeline、single-flight、恢复和 finalize。
- interview-guide：Skill Pack、Provider 配置、结构化输出、统一评估、知识库基础和异步任务。
- TechSpar：长期画像、弱点/强项、SM-2 复习、LangGraph flow 和训练闭环；本项目保留人工审核边界。
- AI-Meeting-Frontend：未来前端面试页、报告页、问答回放、会话恢复和 ASR/TTS 接口契约。
- aural-oss：招聘 SaaS、候选人、邀请、反作弊、代码题和白板能力的远期参考。
- GraphRAG / RAPTOR / HyDE / RAG-Fusion：只作为 Retrieval Harness 设计参考，第一版按轻量、可解释、可回放实现。

## 不做内容

- 不让 Python 直接推进 Go interview session / flow / turn 状态。
- 不让 Python 决定 Provider task routing。
- 不自动把 AI 生成的 memory 写入长期画像，必须保留审核边界。
- 不在业务表里增加持久化锁字段解决 worker 并发。
- 不把 RAG 做成不可解释的普通 topK 向量检索。
- 不在当前阶段扩展招聘 SaaS、组织管理、候选人邀请和反作弊。
