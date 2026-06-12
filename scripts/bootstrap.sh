#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required"
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose v2 is required"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required"
  exit 1
fi

if [ ! -f .env ]; then
  cp .env.example .env
  echo "created .env from .env.example"
fi

docker compose up -d
"$(dirname "$0")/init-db.sh"

mkdir -p .cache/go-build
GOCACHE="$(pwd)/.cache/go-build" go test ./...

echo "bootstrap completed"
echo "run: go run ./cmd/api"
