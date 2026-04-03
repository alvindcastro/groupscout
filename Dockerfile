# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o /groupscout ./cmd/server/main.go

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /groupscout .

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./groupscout"]
