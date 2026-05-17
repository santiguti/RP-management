# syntax=docker/dockerfile:1.6
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && \
    adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=builder /out/api /app/api

EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O- http://localhost:8080/healthz >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/api"]
