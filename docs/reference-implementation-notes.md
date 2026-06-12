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
- Python Runtime 包含结构化 JSON 解析和 Prompt data boundary。

### AI-Meeting

- AI single-flight key 要按业务 stage、session、answer hash 或 business key 设计，避免重复 AI 调用。
- 面试状态机要区分会话状态和题目流转状态。
- Agent 场景绑定要让业务场景和具体 Agent 配置解耦。

当前落地：

- `agent_traces` 已记录 Go 调 Python Runtime 的输入、上下文和输出。
- single-flight 和完整面试状态机尚未落地，后续放在 Go Core，不放在 Python Runtime。

### TechSpar

- Python 侧适合承载 LangGraph/Agent flow、Provider adapter、记忆候选提取。
- 记忆系统要区分知识轴和表现轴，behavior signal 要有限 namespace。
- Profile 更新不能直接全自动覆盖，应该走候选和审核。

当前落地：

- Python Runtime 独立服务已建立。
- Runtime 已预留 `memory_extraction` 任务。
- 长期画像和人工审核还未落地。

## 当前 Go 基础后端完成情况

已完成：

- Gin API 基础。
- PostgreSQL 连接。
- Provider 配置启动同步入库。
- Skill Pack 扫描、热加载、创建、lint。
- Skill Pack 和 references 启动同步入库。
- Context Preview。
- Python Runtime client。
- Go -> Python Runtime dry-run 调用。
- agent trace 落库。
- CodeTop100 / 后端工程题库基础 schema 和查询 API。
- Docker Compose 中间件固定版本和初始化脚本。

未完成：

- Provider 配置的 CRUD API 和加密存储。
- Go 侧 single-flight / idempotency。
- Interview Runtime 状态机。
- Redis 热状态和 PostgreSQL snapshot 恢复。
- Memory candidate / review / profile projection。
- Retrieval Harness 多索引检索。
- 代码执行 judge worker。
