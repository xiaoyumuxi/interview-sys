# AI Interview Platform

个人 AI 面试训练平台后端重写工程。

## 当前阶段

P0/P1 基础环境：

- Go Core API。
- Provider 配置占位和连通性测试。
- Skill Pack 本地扫描。
- Context Preview 调试接口。
- Docker Compose 中间件：PostgreSQL + pgvector、Redis、MinIO。

## 本地启动

```bash
cp .env.example .env
docker compose up -d
go run ./cmd/api
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
- `GET /api/skills/{skill_id}`
- `POST /api/context/preview`

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
