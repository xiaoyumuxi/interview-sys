#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

docker compose up -d postgres

echo "waiting for postgres..."
for _ in $(seq 1 60); do
  if docker compose exec -T postgres pg_isready -U ai_interview -d ai_interview >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

for file in migrations/*.sql; do
  echo "applying ${file}"
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U ai_interview -d ai_interview < "${file}"
done

echo "database initialized"
