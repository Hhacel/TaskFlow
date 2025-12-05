# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files from project root
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the scheduler service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o scheduler ./apps/scheduler

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/scheduler .

# Expose port
EXPOSE 8084

# Run the scheduler service
CMD ["./scheduler"]