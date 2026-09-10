.PHONY: run build test lint swagger tidy docker-up docker-down docker-logs

APP_NAME := mini-bank

run:
	go run ./cmd

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd

test:
	go test ./... -race -cover

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
