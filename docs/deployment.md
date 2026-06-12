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

## Skill 热加载

本地修改 `skills/*/SKILL.md`、`skill.meta.yml` 或 references 后，不需要重启服务，调用：

```bash
curl -s -X POST http://localhost:8080/api/skills/reload
```

也可以通过 `POST /api/skills` 创建新的 Skill Pack。创建接口会写入本地 `skills/{skill_id}` 目录并重新加载 Registry。

## SQL 文件

- `migrations/001_init.sql`：建表、索引、pgvector 扩展。
- `migrations/002_seed_defaults.sql`：默认用户、Provider、任务路由、基础代码题库 seed。

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
