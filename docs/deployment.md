# 本地部署与初始化

## 一键初始化

Mac/Linux:

```bash
./scripts/bootstrap.sh
```

Windows PowerShell:

```powershell
./scripts/bootstrap.ps1
```

脚本会执行：

- 检查 Docker、Docker Compose v2、Go。
- 如果 `.env` 不存在，从 `.env.example` 创建。
- 启动 PostgreSQL + pgvector、Redis、MinIO。
- 应用 `migrations/*.sql`。
- 执行 `go test ./...`。

`bootstrap` 不会自动启动 Go API、独立 worker、Python Runtime 或前端工作台。常用本地进程按需分别启动：

```bash
make run
make run-worker
make run-runtime
make run-frontend
```

前端默认监听 `5173`，并通过 Vite 代理把 `/api` 转发到 Go API。候选人入口支持注册/登录，管理员入口只允许 root；面试异步评估、代码判题、memory review 和 evaluation run 需要对应的 worker / Runtime 服务。

## 认证与工作区

本地开发至少应修改 JWT secret 和 root 密码；默认值只用于首次本地启动：

```text
AUTH_DISABLED=false
JWT_ACCESS_SECRET=local-dev-access-secret-change-me
JWT_REFRESH_SECRET=local-dev-refresh-secret-change-me
ACCESS_TOKEN_TTL_MINUTES=15
REFRESH_TOKEN_TTL_DAYS=30
ROOT_EMAIL=root@example.local
ROOT_PASSWORD=RootChangeMe123!
ROOT_DISPLAY_NAME=Root
```

- API 启动时会按上述配置补齐 root 账号，但不会用默认值把已有账号降级为普通用户。
- `POST /api/auth/register` 只创建普通候选人账号；管理员账号不能从公开注册入口获得。
- `GET /api/interview-sessions`、代码提交和 memory API 按当前认证用户隔离。
- `GET /api/admin/users`、Provider/route、Skill 写操作、Evaluation Harness 和 Ops API 都需要 root。
- `AUTH_DISABLED=true` 仅用于本地诊断；正常开发、测试和部署应保持关闭。
- 前端当前把 access/refresh token 保存在浏览器 `localStorage`。面向不可信网络部署前，应评估 TLS、CSP 与 HttpOnly Cookie/BFF 会话方案。

## Skill 热加载

本地修改 `skills/*/SKILL.md`、`skill.meta.yml` 或 references 后，不需要重启服务，调用：

```bash
curl -s -X POST http://localhost:8080/api/skills/reload \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

也可以通过 `POST /api/skills` 创建新的 Skill Pack。创建接口会写入本地 `skills/{skill_id}` 目录并重新加载 Registry。

`POST /api/skills/reload` 和 `POST /api/skills` 都需要 root token。

## SQL 文件

- `migrations/001_init.sql`：建表、索引、pgvector 扩展。
- `migrations/002_seed_defaults.sql`：默认用户、Provider、任务路由、基础代码题库 seed。

Provider seed 只在记录缺失时插入。模型、base URL、密钥来源和 task route 的运行时切换应通过 Go API 写入数据库，不依赖修改 `.env` 后重启。

API key 有两种来源：

- `env_ref`：数据库保存变量名，例如 `DEEPSEEK_API_KEY`，Go 从 `.env` 读取，适合作为本地 fallback。
- `db_encrypted`：接口提交 `api_key`，Go 使用 `PROVIDER_KEY_ENCRYPTION_SECRET` 做 AES-GCM 加密后写库，适合运行时切换。

未设置 `PROVIDER_KEY_ENCRYPTION_SECRET` 时，接口会拒绝把 API key 写入数据库。

## Coding Judge

代码判题默认关闭，不会执行用户代码：

```text
CODING_JUDGE_ENABLED=false
CODING_JUDGE_MODE=disabled
```

启用本地判题时，先在 `.env` 中设置：

```text
CODING_JUDGE_ENABLED=true
CODING_JUDGE_MODE=docker
```

可选模式：

| Mode | 说明 |
|---|---|
| `disabled` | 默认模式，不执行用户代码，提交会进入 system error / sandbox not configured 路径 |
| `docker` | 每次判题创建临时禁网容器，适合本地隔离验证 |
| `docker_warm` | 复用按语言命名的 stopped container，并通过 tmpfs 回到初始状态 |
| `native_trusted` | 直接调用本机 Go、Java、Python、Node、Deno、g++ 工具链，只适合可信本机代码 |

Docker 模式支持 Go、Java、Python、JavaScript、TypeScript 和 C++ 完整程序。可预拉取默认镜像：

```bash
make pull-judge-images
```

判题 loop 只在 `cmd/worker` 中启动，因此修改 judge 配置后需要运行或重启：

```bash
make run-worker
```

API 进程负责创建提交和查询结果，`cmd/worker` 负责领取 queued submission、执行 evaluator、写入 verdict 和 `code_evaluation_traces`。

## 中间件版本策略

不要用 `latest` 作为默认版本。默认镜像固定为：

```text
POSTGRES_IMAGE=pgvector/pgvector:pg16
REDIS_IMAGE=redis:7-alpine
MINIO_IMAGE=minio/minio:RELEASE.2025-09-07T16-13-09Z
```

已验证 manifest 覆盖：

| Image | amd64 | arm64 |
| --- | --- | --- |
| `pgvector/pgvector:pg16` | yes | yes |
| `redis:7-alpine` | yes | yes |
| `minio/minio:RELEASE.2025-09-07T16-13-09Z` | yes | yes |

运行检查：

```bash
./scripts/check-middleware.sh
```

## 注意

PostgreSQL 官方 entrypoint 只会在数据卷第一次创建时自动执行 `/docker-entrypoint-initdb.d`。如果已有数据卷，修改 migration 后需要手动执行：

```bash
./scripts/init-db.sh
```
