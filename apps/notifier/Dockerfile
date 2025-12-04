# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files from project root
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the notifier service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o notifier ./apps/notifier

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and notification tools
RUN apk --no-cache add ca-certificates curl

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/notifier .

# Expose port
EXPOSE 8083

# Run the notifier service
CMD ["./notifier"]