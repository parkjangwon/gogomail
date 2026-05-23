# syntax=docker/dockerfile:1.7
#
# Multi-stage Dockerfile for gogomail Go backend.
# Produces a minimal distroless image with the gogomail binary and migrations.

# ---- builder ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source and build static binary.
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/gogomail ./cmd/gogomail

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/gogomail /usr/local/bin/gogomail
COPY --from=builder /src/migrations /app/migrations
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

ENV GOGOMAIL_MIGRATION_DIR=/app/migrations \
    APP_MODE=all-in-one \
    TZ=UTC

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/gogomail"]
CMD []
