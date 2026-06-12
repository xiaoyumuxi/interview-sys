.PHONY: run test fmt docker-up docker-down

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
