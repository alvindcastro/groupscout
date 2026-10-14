# Build stage
FROM golang:1.26.9-alpine3.24 AS builder

# Git revision stamped into both binaries and reported by /health.
ARG VERSION=dev

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the applications. Build packages, not single files, so multi-file
# commands (cmd/server is split across several files) compile completely.
RUN go build -ldflags "-X main.version=${VERSION}" -o /groupscout ./cmd/server
RUN go build -ldflags "-X main.version=${VERSION}" -o /alertd ./cmd/alertd

# Final stage
FROM alpine:3.24.2

ARG VERSION=dev

LABEL org.opencontainers.image.title="groupscout" \
      org.opencontainers.image.source="https://github.com/alvindcastro/groupscout" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VERSION}"

RUN apk add --no-cache ca-certificates poppler-utils curl jq \
    && addgroup -S -g 10001 groupscout \
    && adduser -S -D -H -u 10001 -G groupscout groupscout

WORKDIR /app

# Copy the binaries and migrations from the builder stage
COPY --from=builder /groupscout .
COPY --from=builder /alertd .
COPY --from=builder /app/migrations ./migrations

# Run unprivileged; the services only read /app and write temp files to /tmp.
USER groupscout

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./groupscout"]
