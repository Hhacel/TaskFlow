# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files from project root
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the aggregator service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o aggregator ./apps/aggregator

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/aggregator .

# Expose port
EXPOSE 8084

# Run the aggregator service
CMD ["./aggregator"]