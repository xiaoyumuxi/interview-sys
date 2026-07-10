# AI Interview Platform

中文 | [English](./README.en.md)

构建一个可回放、可审计、可恢复的 AI 面试训练平台。

![阶段](https://img.shields.io/badge/stage-workbench%20%2B%20judge-334155)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![Python](https://img.shields.io/badge/Python-3.13-3776AB?logo=python&logoColor=white)
![API](https://img.shields.io/badge/API-Gin-008ECF)
![Runtime](https://img.shields.io/badge/Runtime-FastAPI-009688?logo=fastapi&logoColor=white)
![Queue](https://img.shields.io/badge/Queue-Redis%20Streams-DC382D?logo=redis&logoColor=white)
![Database](https://img.shields.io/badge/Database-PostgreSQL%20%2B%20pgvector-4169E1?logo=postgresql&logoColor=white)

本仓库是个人 AI 面试训练平台重写工程。Go Core API 负责确定性业务事实、状态机、幂等、审计和对外 API；Python AI Runtime 负责模型调用、Prompt 安全、结构化输出、memory 和后续 Agent/RAG 推理；Web Frontend 提供训练工作台，把面试、代码题、记忆审核、管理和评测串成可操作的用户流程。

## 为什么做

- 把模拟面试从同步请求改造成可恢复、可重试的异步运行时。
- 把 Provider、模型、task route 和密钥来源交给 Go 管理，支持运行时切换。
- 保持 Go / Python 边界清晰：业务状态在 Go，非确定性 AI 推理在 Python。
- 用 PostgreSQL 保存业务事实和冷快照，Redis 只做队列、single-flight 和短 TTL 协调。
- 让 dead-letter、worker summary、agent trace 成为可查询的运维事实。

## 系统地图

| 模块 | 路径 | 职责 |
|---|---|---|
| Go API | `cmd/api` | HTTP 入口、鉴权、Provider、Skill、面试会话、代码题、评测和运维 API |
| Worker | `cmd/worker` | Redis Stream 消费、outbox 派发、pending reclaim、dead-letter 和可选 coding judge loop |
| Go 内部包 | `internal` | auth、provider、skill、interview runtime、memory orchestration、context/retrieval、coding、evaluation harness、workqueue、store、routing |
| Web Frontend | `frontend` | Vanilla TypeScript + Monaco 工作台 UI，内置中英文切换，负责训练、会议式面试房间、代码题、memory review、admin、settings 和 evaluation harness 接入 |
| AI Runtime | `python-runtime` | FastAPI task endpoint、Prompt 边界、结构化输出和 memory API |
| 数据库 | `migrations` | PostgreSQL schema、pgvector 扩展和默认 seed |
| Skill Pack | `skills` | 本地技能包，目前包含 `java-backend` |
| 文档 | `docs` | roadmap、职责边界、部署、dead-letter 和参考项目记录 |

## 当前能力

| 能力 | 状态 |
|---|---|
| Auth/User | JWT access + refresh token、bcrypt 密码哈希、root-only 管理接口 |
| Provider | DB 驱动配置、模型切换、任务路由、连通性测试、AES-GCM key 保存 |
| Skill | 本地 Skill Pack 加载、reload、context preview |
| Interview Runtime | session / flow / turn 状态机，answer 提交返回 `202 Accepted` |
| Async pipeline | PostgreSQL local outbox、Redis Stream、独立 worker、pending reclaim |
| Reliability | answer idempotency、Redis single-flight、runtime snapshot、dead-letter |
| Observability | agent traces、dead-letter analysis API、worker summary API |
| Evaluation Harness | root-only 样例集和 run 记录 API，支持 dry-run、断言评分和 agent trace 关联 |
| Coding practice | CodeTop100 / 后端工程题库、OJ 题面字段、异步提交、judge worker、verdict 和 `code_evaluation_traces` |
| Coding completion | `POST /api/coding/completions` 提供确定性题目感知补全画像，覆盖 starter、后端数据化标准库 catalog、局部符号和常见题目模式 |
| Memory orchestration | Go `/api/memory/*` 统一入口，负责鉴权、用户隔离、trace/audit；Python 承载 memory 主逻辑 |
| Memory admission | Context Engine 只把 approved memory 作为 `memory_context` 放入 Prompt，并返回 `memory_admission` 解释 |
| Web Frontend | Vanilla TypeScript + CSS + Monaco 工作台，Vite 代理 `/api`，支持中英文切换、lucide 图标、会议式面试房间、代码题 IDE、OJ 题面规格块、语言草稿、前后端组合轻量联想补全、本地可配置控制条、Companion 面板、状态条、loading/disabled、表单校验、空状态动作和任务导向下一步 |
| Python Runtime | task endpoint、Prompt safety boundary、structured output、memory APIs |
| Middleware | PostgreSQL + pgvector、Redis、MinIO、可选 Python runtime container |

## 环境要求

- Go 1.26 或更高版本
- Python 3.13 或更高版本
- Docker Compose v2
- `uv`，用于本地 Python Runtime 开发
- 目标 Provider 的 API key，或 OpenAI-compatible endpoint

## 快速启动

一键初始化本地中间件、`.env`、数据库 schema、默认 seed 和基础检查：

```bash
make bootstrap
```

手动启动：

```bash
cp .env.example .env
make docker-up
make init-db
make run
make run-worker
```

本地启动 Python Runtime：

```bash
make run-runtime
```

或通过 Docker Compose 启动 Runtime：

```bash
docker compose --profile runtime up -d python-runtime
```

健康检查：

```bash
curl http://localhost:8080/healthz
curl http://localhost:8090/healthz
```

启动前端工作台：

```bash
make run-frontend
```

默认访问 `http://localhost:5173`。如果端口被占用，Vite 会自动尝试下一个可用端口。前端开发服务会把 `/api` 代理到 Go API，因此通常需要同时运行 `make run`；面试异步评估、代码判题、memory review 和 evaluation run 还需要对应的 worker / runtime 服务。

代码判题默认不执行用户代码。需要本地启用时，在 `.env` 中设置 `CODING_JUDGE_ENABLED=true`，并选择 `CODING_JUDGE_MODE=docker`、`docker_warm` 或仅限可信本机代码的 `native_trusted`，然后运行 `make run-worker`。Docker 模式可先用 `make pull-judge-images` 预拉取 Go、Java、Python、JavaScript、TypeScript 和 C++ 镜像。

## 前端工作台

`frontend` 是给用户使用的训练操作台，不是 API 调试集合。它不引入重型框架，主要依赖 Vanilla TypeScript、CSS、Vite、Monaco Editor 和 `lucide` 图标，把后端能力整理成稳定的训练流程。

当前主要交互：

- 工作台概览：展示 API、worker、outbox、judge 和 evaluation run 状态，并给出下一步入口。
- 面试房间：采用会议式布局，包含主舞台、候选人/Runtime 小窗、底部控制条和右侧 Companion 面板。
- 本地可配置控制：麦克风、摄像头、字幕、共享题面、房间 tab 和笔记会保存到 `localStorage`，方便快速启动和关闭前端会话。
- 代码题：题库选择、OJ 题面规格块、Monaco 代码编辑、语言草稿切换、前端局部符号/快捷片段 + Go completion profile 组合补全、异步提交和 verdict 展示。
- 记忆审核：pending candidate 加载、approve/reject，保证只有 approved memory 进入 Prompt。
- 管理与评测：Provider/worker/judge 概览、evaluation case 保存、dry-run 和 run 记录查看。

面试房间里的控制条当前是用户交互状态层，不会直接采集设备或绕过 Go；创建 session、提交答案、轮询 trace、生成 report 和结束会话仍全部通过 Go API 推进。

## 默认登录

API 启动时会补齐本地 root 账号：

```text
ROOT_EMAIL=root@example.local
ROOT_PASSWORD=RootChangeMe123!
ROOT_DISPLAY_NAME=Root
```

获取 access token：

```bash
ACCESS_TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"root@example.local","password":"RootChangeMe123!"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["tokens"]["access_token"])')
```

预览上下文装配：

```bash
curl -s -X POST http://localhost:8080/api/context/preview \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"task_type":"question_generation","skill_id":"java-backend"}'
```

## 开发命令

| 命令 | 说明 |
|---|---|
| `make bootstrap` | 执行 `scripts/bootstrap.sh` |
| `make docker-up` | 启动 PostgreSQL + pgvector、Redis 和 MinIO |
| `make docker-down` | 停止 Docker Compose 中间件 |
| `make init-db` | 应用 SQL migrations 和默认 seed |
| `make run` | 启动 Go Core API，监听 `8080` |
| `make run-worker` | 启动独立 Redis Stream worker |
| `make run-runtime` | 启动 FastAPI Runtime，监听 `8090` |
| `make run-frontend` | 启动 TypeScript 前端开发服务，监听 `5173`，并代理 `/api` 到 Go API |
| `make build-frontend` | 对前端执行 TypeScript 检查并构建静态产物 |
| `make test` | 运行全部 Go 测试，即 `go test ./...` |
| `make test-python` | 运行 Python Runtime 单元测试 |
| `make test-frontend` | 校验生成索引并运行前端补全测试 |
| `make test-scripts` | 运行仓库维护脚本单元测试 |
| `make test-all` | 运行 Go、Python、前端和脚本全部测试 |
| `make check` | 运行全量测试、前端构建、Shell 语法、gofmt 和 go vet |
| `make fmt` | 对 `cmd` 和 `internal` 执行 `gofmt` |
| `make check-middleware` | 检查固定中间件镜像的平台兼容性 |
| `make pull-judge-images` | 预拉取 Docker coding judge 所需语言镜像 |

## CI 流水线

| Workflow | 触发 | 目的 |
|---|---|---|
| `CI` | PR、`main/master` push、手动触发 | 运行 Go race/覆盖率、Python、前端、生成索引与仓库脚本测试，并构建前端 |
| `Quality` | PR、`main/master` push、手动触发 | 检查 `gofmt`、`go mod tidy`、`go vet`、Python compile 和 Compose 配置 |
| `Docker` | Runtime / Compose 相关文件变化、手动触发 | 校验 Docker Compose runtime profile 并构建 Python Runtime 镜像 |
| `Integration Smoke` | migration / middleware / init 脚本变化、手动触发 | 拉起 PostgreSQL、Redis、MinIO，执行 migration/seed 和基础健康检查 |
| `Performance` | 服务、Runtime、migration、压测脚本变化、手动触发 | 运行 Go benchmark，并用 k6 对 Go API 和 Python Runtime 的 `/healthz` 做轻量压测 |

当前性能流水线是 CI 级别的 smoke load，不调用真实模型，也不压外部 Provider。k6 脚本位于 `scripts/k6`，默认 `10 VUs / 30s`，阈值为失败率 `< 1%`、P95 `< 200ms`、检查通过率 `> 99%`。结果会写入 GitHub Actions Summary，以表格展示请求数、RPS、失败率、P95、最大延迟和 Go benchmark 的 `ns/op`、`B/op`、`allocs/op`，原始输出会作为 artifact 保留。

## API Surface

| 分组 | Endpoints |
|---|---|
| Health | `GET /healthz` |
| Auth | `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/refresh`, `POST /api/auth/logout`, `GET /api/auth/me` |
| Providers | `GET/POST /api/providers`, `GET/PUT/DELETE /api/providers/{provider_id}`, `POST /api/providers/test` |
| Routes | `GET /api/provider-routes`, `PUT /api/provider-routes/{task_type}` |
| Skills | `GET/POST /api/skills`, `POST /api/skills/reload`, `GET /api/skills/{skill_id}` |
| Context, Retrieval & Agent | `POST /api/context/preview`, `POST /api/retrieval/search`, `POST /api/agent/tasks` |
| Memory | `GET/POST /api/memory/candidates`, `POST /api/memory/candidates/{candidate_id}/approve`, `POST /api/memory/candidates/{candidate_id}/reject`, `POST /api/memory/candidates/{candidate_id}/edit`, `GET /api/memory/profile`, `GET /api/memory/search`, `GET /api/memory/reviews/due` |
| Interview | `POST /api/interview-sessions`, `GET /api/interview-sessions/{session_id}`, `POST /api/interview-sessions/{session_id}/answers`, `POST /api/interview-sessions/{session_id}/finalize`, `GET /api/interview-sessions/{session_id}/trace`, `GET/POST /api/interview-sessions/{session_id}/report` |
| Coding | `GET /api/coding/question-sets`, `GET /api/coding/questions`, `GET /api/coding/questions/{question_id}`, `POST /api/coding/completions`, `POST /api/coding/submissions`, `GET /api/coding/submissions`, `GET /api/coding/submissions/{submission_id}` |
| Evaluation | `GET/POST /api/evaluation/cases`, `GET /api/evaluation/cases/{case_id}`, `POST /api/evaluation/cases/{case_id}/run`, `GET /api/evaluation/runs` |
| Ops | `GET /api/ops/dead-letters/summary`, `GET /api/ops/dead-letters`, `GET /api/ops/dead-letters/{dead_letter_id}`, `GET /api/ops/workers/summary`, `GET /api/ops/coding-judge/summary` |

Python Runtime:

| 分组 | Endpoints |
|---|---|
| Health | `GET http://localhost:8090/healthz` |
| Tasks | `POST http://localhost:8090/api/runtime/tasks` |
| Memory | `GET/POST http://localhost:8090/api/runtime/memory/candidates`, `POST http://localhost:8090/api/runtime/memory/candidates/{candidate_id}/approve`, `POST http://localhost:8090/api/runtime/memory/candidates/{candidate_id}/reject`, `POST http://localhost:8090/api/runtime/memory/candidates/{candidate_id}/edit` |
| Profile & Review | `GET http://localhost:8090/api/runtime/memory/profile`, `GET http://localhost:8090/api/runtime/memory/search`, `GET http://localhost:8090/api/runtime/reviews/due` |

## 运行时规则

| 主题 | 规则 |
|---|---|
| Session state | `interview_sessions` 区分 `session_status` 和 `flow_status`，合法流转由 Go 校验 |
| Answer submission | `POST /api/interview-sessions/{session_id}/answers` 创建 queued turn 并返回 `202 Accepted` |
| Turn state | `interview_turns.turn_status` 使用 `queued -> running -> completed/failed`，stale running turn 可回到 `queued` |
| Locks | 不新增持久化业务锁字段；并发依赖幂等、`FOR UPDATE SKIP LOCKED`、turn 状态更新和短 TTL Redis 协调 |
| Recovery | PostgreSQL runtime snapshot 保留 Redis 丢失后的业务事实 |
| Final report | `interview_reports` 保存报告状态和内容；Go 聚合确定性事实，Python Runtime `summary` task 只负责生成文本结构 |
| Retrieval harness | `POST /api/retrieval/search` 返回 Skill reference、summary、recent history 和 approved memory 的 evidence、score、reason、source 与 debug trace；vector 暂以 warning 标记未建索引 |
| Evaluation harness | root-only `/api/evaluation/*` 管理样例和运行记录；`expected.required_fields`、`expected.contains`、`expected.equals` 用于可配置断言，`POST /run` 支持 `dry_run` |
| Worker | API 进程负责入队和查询；`cmd/worker` 消费 Redis Stream 事件 |
| Coding judge | `CODING_JUDGE_ENABLED=true` 才会在 `cmd/worker` 中启动 coding judge loop；`CODING_JUDGE_MODE=docker` 每次创建临时禁网容器；`docker_warm` 复用按语言命名的 stopped container，通过 tmpfs 回到初始状态；镜像可配置且可用 `make pull-judge-images` 预拉取；`native_trusted` 直接调用本机工具链，启动快但不隔离，只适合本地可信代码；默认 disabled evaluator 不执行用户代码 |
| Coding completion | `POST /api/coding/completions` 是 Go 内的确定性建议服务，不调用模型、不写数据库；根据语言、题目标签、源码和前缀返回 starter、后端数据化标准库 catalog、局部符号和常见题型模式，作为 Monaco 局部符号/快捷片段的补充，不替代完整 LSP |
| Frontend workbench | `frontend` 是面向用户的操作台，不直接写内部状态；登录后通过 Go API 使用训练工作台、会议式面试房间、代码题、memory review、admin、settings 和 evaluation harness。交互层负责展示系统状态、下一步动作、表单校验、空状态引导、本地可配置会议控件，以及 Monaco 代码题 IDE 的语言草稿和轻量联想补全 |
| Embedded worker | `ENABLE_EMBEDDED_WORKER=true` 仅用于本地兼容模式 |
| Memory context | Context Preview 和 answer evaluation 会按当前 user、task_type、skill、query 和 token budget 引入 approved memory；`memory_extraction` 不引入长期 memory |

## Dead Letter 设计

| 层级 | 目的 |
|---|---|
| Redis Stream dead-letter | 短期缓冲，把 poison message 从主 consumer group 移出 |
| PostgreSQL `dead_letter_events` | 长期标准化事实表，统一收集 Redis poison message 和 outbox 派发失败 |

当前规则：Redis pending message 和 PostgreSQL outbox 派发失败在第 3 次投递或尝试后进入 dead-letter 处理。外部系统应读取 `/api/ops/dead-letters*`，不要依赖 Redis 内部格式。

## Provider 配置

`.env` 只作为 bootstrap 和本地 fallback。Go 会把缺失的默认 Provider seed 到 `provider_configs`，不会覆盖数据库中已经通过 API 修改过的运行时配置。

Provider key 来源：

| 来源 | 使用场景 |
|---|---|
| `env_ref` | 数据库存环境变量名，例如 `DEEPSEEK_API_KEY`；Go 从 `.env` 读取 |
| `db_encrypted` | 通过 API 提交 `api_key`；Go 使用 `PROVIDER_KEY_ENCRYPTION_SECRET` 加密保存 |

未设置 `PROVIDER_KEY_ENCRYPTION_SECRET` 时，不允许把 API key 写入数据库。响应只返回 `api_key_configured`，不会回显原始 key。

## 中间件镜像

| 服务 | 镜像 |
|---|---|
| PostgreSQL + pgvector | `pgvector/pgvector:pg16` |
| Redis | `redis:7-alpine` |
| MinIO | `minio/minio:RELEASE.2025-09-07T16-13-09Z` |

检查镜像 manifest：

```bash
make check-middleware
```

## 文档

| 文档 | 说明 |
|---|---|
| [docs/roadmap.md](./docs/roadmap.md) | 当前计划和下一批任务 |
| [docs/go-python-responsibilities.md](./docs/go-python-responsibilities.md) | Go / Python 职责分工 |
| [docs/language-boundaries.md](./docs/language-boundaries.md) | 业务、Provider 和 runtime 边界 |
| [docs/dead-letter-analysis.md](./docs/dead-letter-analysis.md) | Dead-letter 链路和运维 API |
| [docs/evaluation-harness.md](./docs/evaluation-harness.md) | Evaluation case、断言和 run 记录 |
| [docs/frontend-workbench.md](./docs/frontend-workbench.md) | 前端工作台页面、交互原则和本地开发说明 |
| [docs/python-runtime.md](./docs/python-runtime.md) | Python Runtime API、启动和边界说明 |
| [docs/deployment.md](./docs/deployment.md) | 本地部署和初始化 |
| [docs/reference-projects.md](./docs/reference-projects.md) | 参考项目索引 |
| [docs/reference-implementation-notes.md](./docs/reference-implementation-notes.md) | 参考项目设计点的当前吸收状态 |
| [docs/skill-design-notes.md](./docs/skill-design-notes.md) | Skill Pack 结构、校验和上下文设计原则 |

## 安全说明

- 不要提交真实 API key。
- 本地配置从 `.env.example` 复制到 `.env`。
- Provider 写操作和 Skill 写操作需要 `root` 角色。
- `AUTH_DISABLED=true` 仅用于本地调试，不应作为正常开发、测试或部署依赖。
- Python Runtime 使用 Go 为当前任务传入的 Provider 配置，不持久化 API key，也不绕过 Go 推进业务状态。
