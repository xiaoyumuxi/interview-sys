.PHONY: bootstrap init-db check-middleware run test fmt docker-up docker-down

bootstrap:
	./scripts/bootstrap.sh

init-db:
	./scripts/init-db.sh

check-middleware:
	./scripts/check-middleware.sh

run:
	go run ./cmd/api

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

docker-up:
	docker compose up -d

docker-down:
	docker compose down
