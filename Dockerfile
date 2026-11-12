# ---------- tahap build ----------
FROM golang:1.23-alpine AS build

WORKDIR /src

# Cache dependency dulu supaya rebuild cepat saat kode berubah
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

# Static binary, tanpa CGO
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/server ./cmd/server

# ---------- tahap runtime ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

WORKDIR /app

COPY --from=build /out/server /app/server
COPY static /app/static

USER app

ENV APP_PORT=8181 \
    STATIC_DIR=/app/static

EXPOSE 8181

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8181/api/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/server"]