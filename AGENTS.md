# 仓库协作指南

## 项目结构与模块边界

本仓库是个人 AI 面试训练平台单体工程，由 Go Core API、Python AI Runtime 和 Web Frontend 三部分组成。

- `cmd/api`：Go HTTP API 入口，负责路由、鉴权、Provider、Skill、面试会话、代码题、评测和运维接口。
- `cmd/worker`：Go 后台 worker，消费 Redis Stream，处理 outbox 派发、重试、pending reclaim、dead letter，以及可选 coding judge loop。
- `internal`：Go 业务包，包含 auth、provider、skill、interview runtime、memory orchestration、context/retrieval、coding、evaluation harness、workqueue、store、HTTP routing 等模块。
- `migrations`：PostgreSQL schema 和默认 seed SQL。
- `python-runtime`：FastAPI AI Runtime，负责 LLM 调用、Prompt 安全、结构化输出和 memory API。
- `frontend`：Vanilla TypeScript + Vite + Monaco Editor 工作台，负责登录、会议式面试房间、代码题、memory review、settings 和 evaluation harness UI。
- `skills`：本地 Skill Pack，目前包含 `java-backend`。
- `docs`：路线图、架构、部署、运行时、Go/Python 职责边界和设计说明；当前计划入口是 `docs/roadmap.md`。
- `scripts`：本地 bootstrap、数据库初始化和中间件检查脚本。

## 本地开发命令

- `make bootstrap`：执行 `scripts/bootstrap.sh`，初始化本地环境。
- `make docker-up`：启动 PostgreSQL + pgvector、Redis、MinIO 等 Docker Compose 中间件。
- `make docker-down`：停止 Docker Compose 中间件。
- `make init-db`：应用 SQL migrations 和默认 seed。
- `make check-middleware`：检查中间件镜像版本和平台兼容性。
- `make run`：通过 `go run ./cmd/api` 启动 Go API。
- `make run-worker`：通过 `go run ./cmd/worker` 启动独立 worker。
- `make run-runtime`：在 `python-runtime` 下启动 FastAPI Runtime，监听 `8090`。
- `make run-frontend`：在 `frontend` 下启动 Vite 工作台，默认监听 `5173` 并代理 `/api` 到 Go API。
- `make build-frontend`：对前端执行 TypeScript 检查并构建静态产物。
- `make pull-judge-images`：预拉取 Docker coding judge 所需 Go、Java、Python、JavaScript、TypeScript 和 C++ 镜像。
- `make test`：运行全部 Go 测试，即 `go test ./...`。
- `make test-python`：运行 Python runtime 单元测试。
- `make test-frontend`：校验生成索引并运行前端补全测试。
- `make test-scripts`：运行仓库维护脚本单元测试。
- `make test-all`：运行 Go、Python、前端和脚本的全部测试。
- `make check`：运行全量测试、前端构建、Shell 语法、gofmt 和 go vet 校验。
- `make fmt`：对 `cmd` 和 `internal` 下 Go 代码执行 `gofmt`。

## CI 流水线

仓库使用 GitHub Actions，配置位于 `.github/workflows`。

- `ci.yml`：基础测试流水线，运行 Go race/覆盖率、Python Runtime、前端、生成索引和仓库脚本测试，并构建前端。
- `quality.yml`：质量检查流水线，检查 `gofmt`、`go mod tidy`、`go vet`、Python compile、Shell 语法和 Docker Compose 配置。
- `docker.yml`：Docker 构建流水线，在 runtime / Compose 相关文件变化时构建 Python Runtime 镜像。
- `integration-smoke.yml`：中间件冒烟流水线，拉起 PostgreSQL、Redis、MinIO，执行 migration/seed，并检查基础可用性。
- `performance.yml`：性能冒烟流水线，运行 `go test -run '^$' -bench=. -benchmem ./...`，并用 `scripts/k6` 下的 k6 脚本对 Go API 和 Python Runtime 的 `/healthz` 做轻量压测；默认不调用真实模型和外部 Provider。结果通过 `scripts/perf/perf_summary.py` 写入 GitHub Actions Summary 表格，并上传原始 benchmark/k6 输出 artifact。

## Go / Python 职责边界

Go Core API 负责确定性业务事实、状态推进、幂等、审计和对外 API。凡是会改变业务状态、写数据库主事实、影响 worker 消费、影响请求幂等或审计记录的逻辑，默认放在 Go。

Python AI Runtime 负责非确定性 AI 推理。模型调用、Prompt 细节、结构化输出解析/修复、RAG、Agent flow、memory candidate/review/profile projection 等逻辑默认放在 Python。

关键约束：

- 面试 session / flow / turn 状态机只能由 Go 推进。
- Redis single-flight、业务幂等、outbox、worker claim 和 dead-letter 处理留在 Go。
- Provider 配置、密钥来源、task routing 和连通性测试由 Go 管理。
- Python 只使用 Go 请求传入的 Provider 配置执行模型调用，不决定 task routing。
- Python 不直接写 Go 业务主表，不绕过 Go 推进 interview runtime。
- Go 对外提供 `/api/memory/*` 作为 Python memory API 的统一入口，负责鉴权、用户隔离、trace/audit 和错误标准化；Python 仍负责 memory candidate/review/profile/search/due review 的主逻辑。
- Go Context Engine 负责 memory context admission：只允许 approved memory 以 `memory_context` 形式进入 Prompt，并在 `memory_admission` 中记录 user、query、candidate_ids、reason 和 warning；`memory_extraction` 不引入长期 memory。
- Go 负责代码题题库、提交、completion profile、judge worker 状态推进和判题审计；Python 不执行用户代码，也不决定判题路由。
- Python trace 不记录 API key；Go 负责写 `agent_traces` 等审计事实。
- Python 仅仅负责有关LLM编排、工具调用、记忆管理、Agent和SubAgent的管理等大模型应用层面的逻辑部分，而可靠性等内容都是由Go这类后端业务类型语言负责进行推进的。
- 项目是单体项目，但是设计层面上需要考虑是否方便修改为微服务架构的内容，预计以后可能修改成的微服务有三个模块——模拟面试的Go后端业务模块、集成管理的SaaS对接服务模块、Python运行时。

## 编码风格

Go 代码必须使用 `gofmt`。包名保持短小、小写，并贴合 `internal/<module>` 的既有领域划分。新增 API 响应建议使用明确 schema 版本，例如 `interview.session.v1`。

Python 代码遵循常规 PEP 8 风格，runtime 逻辑放在 `python-runtime/app`。不要把 Go 业务状态推进、Provider 路由、代码判题或主业务表写入逻辑放进 Python。

前端代码使用 TypeScript，入口位于 `frontend/src/main.ts`，API client 位于 `frontend/src/api.ts`，样式集中在 `frontend/src/styles.css`。改动应保持现有 Vanilla TypeScript、Monaco 和 lucide 图标体系，不为局部页面引入重型框架或组件库。

改动应优先沿用现有模块、store、状态机和路由模式。避免为局部需求引入跨边界抽象，除非它能显著减少重复或已经符合仓库既有设计。

## 测试要求

Go 测试使用标准 `testing` 包，文件命名为 `*_test.go`。当前覆盖集中在 auth、provider、skill、interview runtime、context/retrieval、coding judge/completion、evaluation harness 和 memory API 编排周边；新增状态机、outbox、worker、API 行为时应补充聚焦测试。

Python 测试位于 `python-runtime/tests`，命名为 `test_*.py`，通过 `make test-python` 或 `uv run python -m unittest discover -s tests -p 'test_*.py' -v` 运行。

前端改动至少运行 `make build-frontend`。涉及 Go 代码时，至少运行相关包测试；涉及跨模块状态机、worker 或存储行为时优先运行 `make test`。只改文档时无需运行测试，但应确认 Markdown 内容和命令与 Makefile、README、docs 保持一致。

## 面试运行时与异步处理注意事项

- `interview_sessions` 区分 `session_status` 和 `flow_status`，合法流转集中在 Go 状态机里校验。
- Answer 提交接口返回 `202 Accepted`，先创建 queued turn，再由 Redis Stream worker 异步评估。
- `interview_turns` 使用 `turn_status` 记录 `queued/running/completed/failed`，异常抢占可回到 `queued` 重试。
- 不要向表里新增持久化业务锁字段来解决并发；当前设计依赖数据库幂等约束、`FOR UPDATE SKIP LOCKED`、turn 状态更新和短 TTL Redis 协调。
- Redis 丢失后，业务事实应能从 PostgreSQL runtime snapshot 和主表恢复。
- Poison message 超过投递上限后进入 Redis dead-letter stream，并由 worker 标准化写入 PostgreSQL `dead_letter_events`。
- 代码题提交进入 `code_submissions` 后由 `cmd/worker` 中的 coding judge loop 领取；只有 `CODING_JUDGE_ENABLED=true` 才会启动该 loop。
- `POST /api/coding/completions` 是 Go 内的确定性补全画像服务，包含后端数据化标准库 catalog，不调用模型、不写数据库，不替代完整 LSP。

## 配置与安全

不要提交真实密钥。本地配置放在 `.env`，可从 `.env.example` 复制。Provider key 应通过 Go 管理的 Provider 配置流转；Python Runtime 不持久化 API key。

Provider API key 支持两种来源：

- `env_ref`：数据库保存环境变量名，Go 从 `.env` 读取，适合作为本地 fallback。
- `db_encrypted`：接口提交 `api_key`，Go 使用 `PROVIDER_KEY_ENCRYPTION_SECRET` 加密后写库，适合运行时切换。

未设置 `PROVIDER_KEY_ENCRYPTION_SECRET` 时，不应允许把 API key 写入数据库。

本地可用 `AUTH_DISABLED=true` 调试受保护接口，但正常开发、测试和部署不要依赖该配置。Provider 配置和 Skill 写操作需要 `root` 角色。

## 文件追踪与忽略

应追踪源码、SQL migration、seed、脚本、文档、Skill Pack、`README*.md`、各级 `AGENTS.md` 和 `.env.example`。`python-runtime/AGENTS.md` 是子目录协作指南，不应加入忽略规则。

不应追踪真实密钥、本地环境文件、运行时数据、缓存、虚拟环境、覆盖率、日志、临时目录、编辑器配置和构建产物。当前 `.gitignore` 覆盖 `.env`、`.env.*`、`python-runtime/data/`、`.cache/`、`tmp/`、`python-runtime/.venv/`、`python-runtime/.pytest_cache/`、`__pycache__/`、`*.pyc`、`coverage.out`、`.coverage`、`htmlcov/`、`*.test`、`node_modules/`、`bin/`、`dist/`、`build/`、`*.log`、`.DS_Store`、`.idea/` 和 `.vscode/`，并显式保留 `.env.example`。

## 数据库与中间件

SQL 初始化文件：

- `migrations/001_init.sql`：建表、索引和 pgvector 扩展。
- `migrations/002_seed_defaults.sql`：默认用户、Provider、任务路由和基础代码题库 seed。

PostgreSQL 官方 entrypoint 只会在数据卷首次创建时执行初始化 SQL。已有数据卷下修改 migration 后，需要手动运行 `make init-db`。

默认中间件镜像不要使用 `latest`，保持与文档一致的固定版本，引入的中间件的版本需要是跨平台可以一键下载的：

- `pgvector/pgvector:pg16`
- `redis:7-alpine`
- `minio/minio:RELEASE.2025-09-07T16-13-09Z`

## Commit 与 Pull Request

提交信息使用简短祈使句，和现有风格一致，例如 `Add worker metrics summary API`。PR 应包含简要 summary、测试结果、schema/migration 说明，以及 API contract 变化。涉及 roadmap 或 issue 时附上关联链接。
