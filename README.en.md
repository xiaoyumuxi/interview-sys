# AI Interview Platform

[中文](./README.md) | English

Build a replayable and auditable AI interview backend.

![Stage](https://img.shields.io/badge/stage-async%20runtime%20%2B%20memory-334155)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![Python](https://img.shields.io/badge/Python-3.13-3776AB?logo=python&logoColor=white)
![API](https://img.shields.io/badge/API-Gin-008ECF)
![Runtime](https://img.shields.io/badge/Runtime-FastAPI-009688?logo=fastapi&logoColor=white)
![Queue](https://img.shields.io/badge/Queue-Redis%20Streams-DC382D?logo=redis&logoColor=white)
![Database](https://img.shields.io/badge/Database-PostgreSQL%20%2B%20pgvector-4169E1?logo=postgresql&logoColor=white)

This repository is a backend rewrite for a personal AI interview training platform. Go Core API owns deterministic business facts, state transitions, idempotency, audit and external APIs. Python AI Runtime owns model calls, prompt safety, structured output, memory and later Agent/RAG reasoning.

## Why This Exists

- Turn mock interviews from synchronous requests into a recoverable and retryable async runtime.
- Manage providers, models, task routes and key sources in Go so they can be changed at runtime.
- Keep the Go / Python boundary clear: business state in Go, non-deterministic AI reasoning in Python.
- Store business facts and cold snapshots in PostgreSQL; use Redis only for queues, single-flight and short TTL coordination.
- Make dead-letter events, worker summaries and agent traces queryable operations facts.

## System Map

| Area | Path | Role |
|---|---|---|
| Go API | `cmd/api` | HTTP entrypoint, auth, providers, skills, interview sessions and ops APIs |
| Worker | `cmd/worker` | Redis Stream consumption, outbox dispatch, pending reclaim and dead-letter handling |
| Go internals | `internal` | auth, provider, skill, interview runtime, memory orchestration, workqueue, store and routing |
| Web Frontend | `frontend` | Vanilla TypeScript workbench UI with Chinese/English switching for training, coding, memory review, admin and evaluation harness flows |
| AI Runtime | `python-runtime` | FastAPI task endpoint, prompt boundaries, structured output and memory APIs |
| Database | `migrations` | PostgreSQL schema, pgvector extension and default seed data |
| Skill packs | `skills` | Local skill packs, currently `java-backend` |
| Docs | `docs` | roadmap, boundaries, deployment, dead-letter analysis and reference notes |

## Current Capabilities

| Capability | Status |
|---|---|
| Auth/User | JWT access + refresh token, bcrypt password hashing, root-only management APIs |
| Provider | DB-driven config, model switching, task routing, connectivity tests and AES-GCM key storage |
| Skill | Local skill pack loading, reload and context preview |
| Interview Runtime | session / flow / turn state machine; answer submission returns `202 Accepted` |
| Async pipeline | PostgreSQL local outbox, Redis Stream, standalone worker and pending reclaim |
| Reliability | answer idempotency, Redis single-flight, runtime snapshot and dead-letter handling |
| Observability | agent traces, dead-letter analysis API and worker summary API |
| Evaluation Harness | root-only case and run APIs with dry-run, assertion scoring and agent trace linkage |
| Memory orchestration | Go `/api/memory/*` entrypoint for auth, user isolation and trace/audit; Python owns memory logic |
| Memory admission | Context Engine admits only approved memory as `memory_context` and returns a `memory_admission` explanation |
| Web Frontend | Vanilla TypeScript + CSS, Vite dev proxy for `/api`, and configurable Chinese/English UI language |
| Python Runtime | task endpoint, prompt safety boundary, structured output and memory APIs |
| Middleware | PostgreSQL + pgvector, Redis, MinIO and optional Python runtime container |

## Requirements

- Go 1.26 or later
- Python 3.13 or later
- Docker Compose v2
- `uv` for local Python runtime development
- Provider API key for the selected model, or an OpenAI-compatible endpoint

## Quick Start

Bootstrap local middleware, `.env`, database schema, default seed and basic checks:

```bash
make bootstrap
```

Manual startup:

```bash
cp .env.example .env
make docker-up
make init-db
make run
make run-worker
```

Start Python Runtime locally:

```bash
make run-runtime
```

Or run the runtime through Docker Compose:

```bash
docker compose --profile runtime up -d python-runtime
```

Health checks:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8090/healthz
```

## Default Login

The API bootstraps a local root account when missing:

```text
ROOT_EMAIL=root@example.local
ROOT_PASSWORD=RootChangeMe123!
ROOT_DISPLAY_NAME=Root
```

Get an access token:

```bash
ACCESS_TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"root@example.local","password":"RootChangeMe123!"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["tokens"]["access_token"])')
```

Preview context assembly:

```bash
curl -s -X POST http://localhost:8080/api/context/preview \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"task_type":"question_generation","skill_id":"java-backend"}'
```

## Developer Commands

| Command | Description |
|---|---|
| `make bootstrap` | Run `scripts/bootstrap.sh` |
| `make docker-up` | Start PostgreSQL + pgvector, Redis and MinIO |
| `make docker-down` | Stop Docker Compose middleware |
| `make init-db` | Apply SQL migrations and default seed |
| `make run` | Start Go Core API on `8080` |
| `make run-worker` | Start the standalone Redis Stream worker |
| `make run-runtime` | Start FastAPI Runtime on `8090` |
| `make run-frontend` | Start the TypeScript frontend dev server on `5173` with `/api` proxied to Go API |
| `make build-frontend` | Type-check and build the frontend static assets |
| `make test` | Run all Go tests with `go test ./...` |
| `make test-python` | Run Python runtime unit tests |
| `make fmt` | Run `gofmt` for `cmd` and `internal` |
| `make check-middleware` | Check pinned middleware image compatibility |

## CI Pipelines

| Workflow | Trigger | Purpose |
|---|---|---|
| `CI` | PRs, `main/master` pushes and manual dispatch | Run `go test ./...` and Python Runtime unit tests |
| `Quality` | PRs, `main/master` pushes and manual dispatch | Check `gofmt`, `go mod tidy`, `go vet`, Python compile and Compose config |
| `Docker` | Runtime / Compose related changes and manual dispatch | Validate the runtime Compose profile and build the Python Runtime image |
| `Integration Smoke` | Migration / middleware / init script changes and manual dispatch | Start PostgreSQL, Redis and MinIO, then apply migrations/seed and run basic health checks |
| `Performance` | Service, runtime, migration, load-test script changes and manual dispatch | Run Go benchmarks and use k6 for lightweight `/healthz` load tests against Go API and Python Runtime |

The current performance workflow is CI-level smoke load. It does not call real models or external providers. k6 scripts live in `scripts/k6`; defaults are `10 VUs / 30s` with thresholds of failure rate `< 1%`, P95 `< 200ms` and check pass rate `> 99%`. Results are rendered into GitHub Actions Summary tables with request count, RPS, failure rate, P95, max latency and Go benchmark `ns/op`, `B/op`, `allocs/op`; raw outputs are kept as artifacts.

## API Surface

| Group | Endpoints |
|---|---|
| Health | `GET /healthz` |
| Auth | `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/refresh`, `POST /api/auth/logout`, `GET /api/auth/me` |
| Providers | `GET/POST /api/providers`, `GET/PUT/DELETE /api/providers/{provider_id}`, `POST /api/providers/test` |
| Routes | `GET /api/provider-routes`, `PUT /api/provider-routes/{task_type}` |
| Skills | `GET/POST /api/skills`, `POST /api/skills/reload`, `GET /api/skills/{skill_id}` |
| Context, Retrieval & Agent | `POST /api/context/preview`, `POST /api/retrieval/search`, `POST /api/agent/tasks` |
| Memory | `GET/POST /api/memory/candidates`, `POST /api/memory/candidates/{candidate_id}/approve`, `POST /api/memory/candidates/{candidate_id}/reject`, `POST /api/memory/candidates/{candidate_id}/edit`, `GET /api/memory/profile`, `GET /api/memory/search`, `GET /api/memory/reviews/due` |
| Interview | `POST /api/interview-sessions`, `GET /api/interview-sessions/{session_id}`, `POST /api/interview-sessions/{session_id}/answers`, `POST /api/interview-sessions/{session_id}/finalize`, `GET /api/interview-sessions/{session_id}/trace`, `GET/POST /api/interview-sessions/{session_id}/report` |
| Coding | `GET /api/coding/question-sets`, `GET /api/coding/questions`, `GET /api/coding/questions/{question_id}`, `POST /api/coding/submissions`, `GET /api/coding/submissions`, `GET /api/coding/submissions/{submission_id}` |
| Evaluation | `GET/POST /api/evaluation/cases`, `GET /api/evaluation/cases/{case_id}`, `POST /api/evaluation/cases/{case_id}/run`, `GET /api/evaluation/runs` |
| Ops | `GET /api/ops/dead-letters/summary`, `GET /api/ops/dead-letters`, `GET /api/ops/dead-letters/{dead_letter_id}`, `GET /api/ops/workers/summary`, `GET /api/ops/coding-judge/summary` |

Python Runtime:

| Group | Endpoints |
|---|---|
| Health | `GET http://localhost:8090/healthz` |
| Tasks | `POST http://localhost:8090/api/runtime/tasks` |
| Memory | `GET/POST http://localhost:8090/api/runtime/memory/candidates`, `POST http://localhost:8090/api/runtime/memory/candidates/{candidate_id}/approve`, `POST http://localhost:8090/api/runtime/memory/candidates/{candidate_id}/reject`, `POST http://localhost:8090/api/runtime/memory/candidates/{candidate_id}/edit` |
| Profile & Review | `GET http://localhost:8090/api/runtime/memory/profile`, `GET http://localhost:8090/api/runtime/memory/search`, `GET http://localhost:8090/api/runtime/reviews/due` |

## Runtime Rules

| Topic | Rule |
|---|---|
| Session state | `interview_sessions` separates `session_status` and `flow_status`; legal transitions are validated in Go |
| Answer submission | `POST /api/interview-sessions/{session_id}/answers` creates a queued turn and returns `202 Accepted` |
| Turn state | `interview_turns.turn_status` uses `queued -> running -> completed/failed`; stale running turns may return to `queued` |
| Locks | No persistent business lock columns; concurrency relies on idempotency, `FOR UPDATE SKIP LOCKED`, turn state updates and short TTL Redis coordination |
| Recovery | PostgreSQL runtime snapshots preserve business facts after Redis loss |
| Final report | `interview_reports` stores report status and content; Go aggregates deterministic facts and Python Runtime only generates structured summary text |
| Retrieval harness | `POST /api/retrieval/search` returns evidence, score, reason, source and debug trace across skill references, summaries, recent history and approved memory |
| Evaluation harness | root-only `/api/evaluation/*` manages cases and run records; `expected.required_fields`, `expected.contains` and `expected.equals` define configurable assertions, and `POST /run` supports `dry_run` |
| Worker | The API process enqueues and queries; `cmd/worker` consumes Redis Stream events |
| Coding judge | `CODING_JUDGE_ENABLED=true` starts the coding judge loop in `cmd/worker`; `docker`, `docker_warm`, and `native_trusted` modes are configurable |
| Embedded worker | `ENABLE_EMBEDDED_WORKER=true` is only for local compatibility mode |
| Memory context | Context Preview and answer evaluation admit approved memory by user, task_type, skill, query and token budget; `memory_extraction` does not admit long-term memory |

## Dead Letter Design

| Layer | Purpose |
|---|---|
| Redis Stream dead-letter | Short-term buffer for poison messages moved out of the main consumer group |
| PostgreSQL `dead_letter_events` | Long-term standardized fact table for Redis poison messages and outbox dispatch failures |

Current rule: Redis pending messages and PostgreSQL outbox dispatch failures enter dead-letter handling after 3 deliveries or attempts. External systems should read `/api/ops/dead-letters*` instead of depending on Redis internals.

## Provider Configuration

`.env` is only bootstrap and local fallback. Go seeds missing default providers into `provider_configs` and never overwrites runtime changes already stored in the database.

Provider key sources:

| Source | Use case |
|---|---|
| `env_ref` | Store an environment variable name such as `DEEPSEEK_API_KEY`; Go reads it from `.env` |
| `db_encrypted` | Submit `api_key` through the API; Go encrypts it with `PROVIDER_KEY_ENCRYPTION_SECRET` |

If `PROVIDER_KEY_ENCRYPTION_SECRET` is not set, API keys must not be written into the database. Responses only expose `api_key_configured`; raw keys are never echoed.

## Middleware Images

| Service | Image |
|---|---|
| PostgreSQL + pgvector | `pgvector/pgvector:pg16` |
| Redis | `redis:7-alpine` |
| MinIO | `minio/minio:RELEASE.2025-09-07T16-13-09Z` |

## Documentation

| Document | Description |
|---|---|
| [docs/roadmap.md](./docs/roadmap.md) | Current plan and next implementation batch |
| [docs/go-python-responsibilities.md](./docs/go-python-responsibilities.md) | Go / Python responsibility split |
| [docs/language-boundaries.md](./docs/language-boundaries.md) | Business, Provider and runtime boundaries |
| [docs/dead-letter-analysis.md](./docs/dead-letter-analysis.md) | Dead-letter pipeline and ops API |
| [docs/evaluation-harness.md](./docs/evaluation-harness.md) | Evaluation cases, assertions and run records |
| [docs/deployment.md](./docs/deployment.md) | Local deployment and initialization |
| [docs/reference-projects.md](./docs/reference-projects.md) | Reference project index |
