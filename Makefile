.PHONY: build up down test clean proto migrate

# Docker compose commands
up:
	docker-compose up -d

down:
	docker-compose down

build:
	docker-compose build

logs:
	docker-compose logs -f

# Development commands
test:
	go test ./... -cover

clean:
	docker-compose down -v
	docker system prune -f

# Protocol buffers
proto:
	protoc --go_out=. --go-grpc_out=. proto/*.proto

# Database migrations
migrate-up:
	migrate -path migrations -database "postgres://taskflow:taskflow@localhost:5432/taskflow?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://taskflow:taskflow@localhost:5432/taskflow?sslmode=disable" down

# Install dependencies
deps:
	go mod tidy
	go mod download

# Linting
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
