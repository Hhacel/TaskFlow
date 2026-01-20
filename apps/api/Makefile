# Makefile for TaskFlow Scheduler

.PHONY: docs build run test clean

# Generate API code from OpenAPI spec
generate:
	@echo Generating API code from OpenAPI specification...
	@where oapi-codegen >nul 2>&1 || (echo Installing oapi-codegen... && go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest)
	@if not exist internal\api mkdir internal\api
	oapi-codegen -config oapi-codegen.yaml api/taskflow.yaml
	@echo API code generated in internal/api/

# Generate types only
types:
	@echo Generating types from OpenAPI specification...
	@where oapi-codegen >nul 2>&1 || (echo Installing oapi-codegen... && go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest)
	@if not exist internal\api mkdir internal\api
	oapi-codegen -generate types -package api api/openapi.yaml > internal/api/types.go

# Generate server code
server:
	@echo Generating server code from OpenAPI specification...
	@where oapi-codegen >nul 2>&1 || (echo Installing oapi-codegen... && go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest)
	@if not exist internal\api mkdir internal\api
	oapi-codegen -generate gin -package api api/openapi.yaml > internal/api/server.go

# Build the scheduler binary
build:
	@echo Building scheduler service...
	@if not exist bin mkdir bin
	go build -o bin/scheduler.exe ./cmd/main.go

# Run the scheduler service
run:
	@echo Starting scheduler service...
	go run ./cmd/main.go

# Run tests
test:
	@echo Running tests...
	go test ./...

# Clean generated files
clean:
	@echo Cleaning up...
	@if exist internal\api rmdir /s /q internal\api
	@if exist bin rmdir /s /q bin
	@if exist internal\server_impl.go del internal\server_impl.go

# Install dependencies
deps:
	@echo Installing dependencies...
	go mod tidy
	go mod download

# Format code
fmt:
	@echo Formatting code...
	go fmt ./...

# Lint code
lint:
	@echo Linting code...
	@where golangci-lint >nul 2>&1 || (echo Installing golangci-lint... && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# Help
help:
	@echo "Available commands:"
	@echo "  generate - Generate API code from OpenAPI spec"
	@echo "  types    - Generate types only"
	@echo "  server   - Generate server code only"
	@echo "  build    - Build the scheduler binary"
	@echo "  run      - Run the scheduler service"
	@echo "  test     - Run tests"
	@echo "  clean    - Clean generated files"
	@echo "  deps     - Install dependencies"
	@echo "  fmt      - Format code"
	@echo "  lint     - Lint code"
	@echo "  help     - Show this help message"