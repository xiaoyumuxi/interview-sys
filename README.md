# AI Interview Platform

个人 AI 面试训练平台后端重写工程。

## 当前阶段

P0/P1 基础环境：

- Go Core API。
- HTTP framework: Gin。
- Provider 配置占位和连通性测试。
- Skill Pack 本地扫描。
- Context Preview 调试接口。
- Docker Compose 中间件：PostgreSQL + pgvector、Redis、MinIO。
- Python AI Runtime 基础骨架。

## 本地启动

傻瓜式初始化：

```bash
./scripts/bootstrap.sh
```

Windows PowerShell：

```powershell
./scripts/bootstrap.ps1
```

手动启动：

```bash
cp .env.example .env
docker compose up -d
./scripts/init-db.sh
go run ./cmd/api
```

启动 Python Runtime：

```bash
docker compose --profile runtime up -d python-runtime
```

本地运行 Python Runtime：

```bash
cd python-runtime
uv sync
uv run uvicorn app.main:app --host 0.0.0.0 --port 8090
```

健康检查：

```bash
curl http://localhost:8080/healthz
```

Context Preview：

```bash
curl -s -X POST http://localhost:8080/api/context/preview \
  -H 'Content-Type: application/json' \
  -d '{"task_type":"question_generation","skill_id":"java-backend"}'
```

## API

- `GET /healthz`
- `GET /api/providers`
- `POST /api/providers/test`
- `GET /api/skills`
- `POST /api/skills`
- `POST /api/skills/reload`
- `GET /api/skills/{skill_id}`
- `POST /api/context/preview`
- `POST /api/agent/tasks`
- `GET /api/coding/question-sets`
- `GET /api/coding/questions`
- `GET /api/coding/questions/{question_id}`

Python Runtime:

- `GET http://localhost:8090/healthz`
- `POST http://localhost:8090/api/runtime/tasks`

## SQL 初始化

建表 SQL 在 [migrations/001_init.sql](/Users/yaoyao/Documents/SelfProject/migrations/001_init.sql)，默认数据在 [migrations/002_seed_defaults.sql](/Users/yaoyao/Documents/SelfProject/migrations/002_seed_defaults.sql)。

重新应用初始化脚本：

```bash
./scripts/init-db.sh
```

## 中间件版本

默认固定为同时支持 `linux/amd64` 和 `linux/arm64` 的镜像：

```text
pgvector/pgvector:pg16
redis:7-alpine
minio/minio:RELEASE.2025-09-07T16-13-09Z
```

检查 manifest：

```bash
./scripts/check-middleware.sh
```

## Provider 初始化

DeepSeek 默认使用 OpenAI-compatible 格式：

```text
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_CHAT_ENDPOINT_PATH=/chat/completions
DEEPSEEK_CHAT_MODEL=deepseek-v4-flash
```

普通 OpenAI-compatible Provider 默认使用：

```text
OPENAI_COMPATIBLE_BASE_URL=https://api.openai.com/v1
OPENAI_COMPATIBLE_CHAT_ENDPOINT_PATH=/chat/completions
```
