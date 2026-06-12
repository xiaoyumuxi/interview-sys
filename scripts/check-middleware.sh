#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

docker compose config --images
docker buildx imagetools inspect "${POSTGRES_IMAGE:-pgvector/pgvector:pg16}" | grep -E "linux/(amd64|arm64)" || true
docker buildx imagetools inspect "${REDIS_IMAGE:-redis:7-alpine}" | grep -E "linux/(amd64|arm64)" || true
docker buildx imagetools inspect "${MINIO_IMAGE:-minio/minio:RELEASE.2025-09-07T16-13-09Z}" | grep -E "linux/(amd64|arm64)" || true
