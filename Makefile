.PHONY: run build test test-integration lint swagger tidy docker-up docker-down docker-logs

APP_NAME := mini-bank

run:
	go run ./cmd

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd

test:
	go test ./... -race -cover

# Spins up a real Postgres via testcontainers-go and exercises the full
# HTTP-to-database stack, including concurrent transfers. Requires Docker.
test-integration:
	go test -tags=integration ./test/integration/... -race -v -timeout 5m

vet:
	go vet ./...

swagger:
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/main.go -o docs

tidy:
	go mod tidy

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v

docker-logs:
	docker compose logs -f app
