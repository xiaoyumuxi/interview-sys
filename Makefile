.PHONY: bootstrap init-db check-middleware pull-judge-images run run-worker run-runtime run-frontend build-frontend test test-python fmt docker-up docker-down

bootstrap:
	./scripts/bootstrap.sh

init-db:
	./scripts/init-db.sh

check-middleware:
	./scripts/check-middleware.sh

pull-judge-images:
	./scripts/pull-judge-images.sh

run:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

run-runtime:
	cd python-runtime && uv run uvicorn app.main:app --host 0.0.0.0 --port 8090

run-frontend:
	cd frontend && npm run dev

build-frontend:
	cd frontend && npm run build

test:
	go test ./...

test-python:
	cd python-runtime && uv run python -m unittest discover -s tests -p 'test_*.py' -v

fmt:
	gofmt -w ./cmd ./internal

docker-up:
	docker compose up -d

docker-down:
	docker compose down
