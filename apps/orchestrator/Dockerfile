# Build stage
FROM golang:1.25 AS builder

WORKDIR /app

# Copy go mod files from project root
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the orchestrator service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o orchestrator ./apps/orchestrator/cmd/

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/orchestrator .

# Expose port
EXPOSE 8084

# Run the orchestrator service
CMD ["./orchestrator"]
