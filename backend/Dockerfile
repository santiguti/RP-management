# syntax=docker/dockerfile:1.6
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/jobs ./cmd/jobs && \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/goose github.com/pressly/goose/v3/cmd/goose

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && \
    adduser -D -u 10001 app
WORKDIR /app
COPY --from=builder /out/api /out/jobs /out/goose /app/
# Migrations ship in the image so prod can run goose without a Go toolchain:
#   docker compose exec api /app/goose -dir /app/migrations postgres "$DATABASE_URL" up
COPY --from=builder /src/internal/db/migrations /app/migrations
# Pre-create the attachments dir owned by the app user so the named volume
# mounted here inherits writable ownership on first use.
RUN mkdir -p /app/data/attachments && chown -R app:app /app/data
USER app

EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O- http://localhost:8080/healthz >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/api"]
