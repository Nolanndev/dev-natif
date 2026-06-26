# syntax=docker/dockerfile:1

# ---- Build stage -----------------------------------------------------------
# Pure-Go SQLite (modernc.org/sqlite) => CGO disabled => fully static binary.
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Resolve dependencies first for better layer caching.
COPY go.mod go.sum* ./
RUN go mod download || true

# Build.
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- Runtime stage ---------------------------------------------------------
FROM alpine:3.20

# ca-certificates for registry TLS when pulling images.
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app && \
    mkdir -p /data && chown app:app /data

COPY --from=builder /out/api /usr/local/bin/api

# Persistent volume for the SQLite database.
VOLUME ["/data"]
EXPOSE 8080

ENV NATIF_PORT=8080 \
    NATIF_DB_PATH=/data/natif.db \
    NATIF_LOG_LEVEL=info

# Note: the container must be able to reach the Docker Engine socket, mounted at
# /var/run/docker.sock (see docker-compose.yml). Running as a non-root user
# requires that user to be in the docker group of the host socket; for the MVP we
# keep root to guarantee socket access. Override with --user in hardened setups.
USER root

ENTRYPOINT ["/usr/local/bin/api"]
