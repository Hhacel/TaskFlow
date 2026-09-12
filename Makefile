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
	@powershell -Command "$$packages = go list ./... | Where-Object { $$_ -notmatch '/cmd$$' -and $$_ -notmatch '/pkg/persistence$$' }; go test $$packages -cover"

# Run unit tests only (fast)
test-unit:
	@powershell -Command "$$packages = go list ./... | Where-Object { $$_ -notmatch '/cmd$$' -and $$_ -notmatch '/pkg/persistence$$' }; go test $$packages -short -cover"

# Persistence integration tests (requires PostgreSQL)
test-persistence:
	@echo "Starting test database..."
	docker-compose -f docker-compose.test.yaml up -d
	@echo "Waiting for database to be ready..."
	@powershell -Command "Start-Sleep -Seconds 5"
	@echo "Running persistence tests..."
	@powershell -Command "$$env:TEST_DB_HOST='localhost'; $$env:TEST_DB_PORT='5433'; $$env:TEST_DB_USER='taskflow'; $$env:TEST_DB_PASSWORD='taskflow'; $$env:TEST_DB_NAME='taskflow_test'; go test ./pkg/persistence/... -v -cover"
	@echo "Stopping test database..."
	docker-compose -f docker-compose.test.yaml down

# Run all tests including integration tests
test-all: test test-persistence

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
