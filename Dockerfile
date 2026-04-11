# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the applications
RUN go build -o /groupscout ./cmd/server/main.go
RUN go build -o /alertd ./cmd/alertd/main.go

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates poppler-utils curl jq

WORKDIR /app

# Copy the binaries and migrations from the builder stage
COPY --from=builder /groupscout .
COPY --from=builder /alertd .
COPY --from=builder /app/migrations ./migrations

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./groupscout"]
