#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

docker compose config --quiet

check_platforms() {
  local image="$1"
  local inspection

  echo "checking ${image}"
  inspection="$(docker buildx imagetools inspect "${image}")"
  for platform in linux/amd64 linux/arm64; do
    if ! grep -q "${platform}" <<<"${inspection}"; then
      echo "${image} does not publish required platform ${platform}" >&2
      return 1
    fi
  done
}

check_platforms "${POSTGRES_IMAGE:-pgvector/pgvector:pg16}"
check_platforms "${REDIS_IMAGE:-redis:7-alpine}"
check_platforms "${MINIO_IMAGE:-minio/minio:RELEASE.2025-09-07T16-13-09Z}"
