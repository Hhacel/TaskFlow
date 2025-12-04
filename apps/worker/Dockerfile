# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files from project root
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the worker service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker ./apps/worker

# Final stage
FROM alpine:latest

# Install ca-certificates and common tools for task execution
RUN apk --no-cache add ca-certificates curl wget

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/worker .

# Expose port
EXPOSE 8082

# Run the worker service
CMD ["./worker"]