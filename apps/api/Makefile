# Makefile for TaskFlow API Gateway

.PHONY: build run test clean deps fmt lint help

# Build the api binary
build:
	@echo Building api service...
	@if not exist bin mkdir bin
	go build -o bin/api.exe ./cmd/main.go

# Run the api service
run:
	@echo Starting api service...
	go run ./cmd/main.go

# Run tests
test:
	@echo Running tests...
	go test ./...

# Clean build artifacts
clean:
	@echo Cleaning up...
	@if exist bin rmdir /s /q bin

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
	@echo "  build - Build the api binary"
	@echo "  run   - Run the api service"
	@echo "  test  - Run tests"
	@echo "  clean - Clean build artifacts"
	@echo "  deps  - Install dependencies"
	@echo "  fmt   - Format code"
	@echo "  lint  - Lint code"
	@echo "  help  - Show this help message"
