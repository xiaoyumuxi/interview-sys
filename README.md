# AI Interview Platform

个人 AI 面试训练平台后端重写工程。

## 当前阶段

P0/P1 基础环境：

- Go Core API。
- HTTP framework: Gin。
- DB 驱动的 Provider 配置、模型切换和连通性测试。
- Skill Pack 本地扫描。
- Context Preview 调试接口。
- Docker Compose 中间件：PostgreSQL + pgvector、Redis、MinIO。
- Python AI Runtime 基础骨架。
- Go / Python 边界说明：[docs/language-boundaries.md](/Users/yaoyao/Documents/SelfProject/docs/language-boundaries.md)

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
- `POST /api/providers`
- `GET /api/providers/{provider_id}`
- `PUT /api/providers/{provider_id}`
- `DELETE /api/providers/{provider_id}`
- `POST /api/providers/test`
- `GET /api/provider-routes`
- `PUT /api/provider-routes/{task_type}`
- `GET /api/skills`
- `POST /api/skills`
- `POST /api/skills/reload`
- `GET /api/skills/{skill_id}`
- `POST /api/context/preview`
- `POST /api/agent/tasks`
- `POST /api/interview-sessions`
- `GET /api/interview-sessions/{session_id}`
- `POST /api/interview-sessions/{session_id}/answers`
- `POST /api/interview-sessions/{session_id}/finalize`
- `GET /api/interview-sessions/{session_id}/trace`
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

## Interview Runtime

Go 维护面试运行时状态机和幂等边界：

- `interview_sessions` 区分 `session_status` 和 `flow_status`。
- `interview_turns` 用 `request_id` 与 `session_id + question_number + answer_round + answer_hash` 做重复提交回放。
- `interview_runtime_snapshots` 保存 PostgreSQL 冷快照，Redis 丢失后仍能恢复业务事实。
- Redis single-flight 折叠同一答案的重复 AI 评估调用。
- Redis Stream `INTERVIEW_EVENTS_STREAM` 记录 session/answer/finalize 事件，后续 judge、report、memory extraction 可以拆成 consumer group 异步处理。

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

`.env` 只作为 bootstrap 和本地 fallback。Go 启动时会把缺失的默认 Provider seed 到 `provider_configs`，不会覆盖数据库里已经通过 API 修改过的模型、base URL、密钥来源或任务路由。

运行时切换 Provider/model 走数据库 API：

```bash
curl -s -X PUT http://localhost:8080/api/providers/deepseek-default \
  -H 'Content-Type: application/json' \
  -d '{"provider_type":"deepseek","base_url":"https://api.deepseek.com","chat_endpoint_path":"/chat/completions","chat_model":"deepseek-v4-flash","api_key_ref":"DEEPSEEK_API_KEY","supports_streaming":true,"supports_json":true,"enabled":true}'

curl -s -X PUT http://localhost:8080/api/provider-routes/question_generation \
  -H 'Content-Type: application/json' \
  -d '{"provider_id":"deepseek-default","fallback_provider_id":"openai-compatible-default"}'
```

如果要把 API key 写入数据库，必须设置 `PROVIDER_KEY_ENCRYPTION_SECRET`，接口会用 AES-GCM 加密后保存；响应只返回 `api_key_configured`，不会回显 key。未设置加密 secret 时，请使用 `api_key_ref` 指向 `.env` 中的 fallback 变量。

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
