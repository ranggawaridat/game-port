# ---------- tahap build ----------
FROM golang:1.23-alpine AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/cmd ./cmd
COPY backend/internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/server ./cmd/server

# ---------- tahap runtime ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

WORKDIR /app

COPY --from=build /out/server /app/gameport
COPY frontend /app/static

# Folder lama untuk gambar yang diunggah sebelum fitur upload dihapus.
RUN mkdir -p /app/uploads && chown -R app:app /app

USER app

ENV APP_HOST=0.0.0.0 \
    APP_PORT=8181 \
    STATIC_DIR=/app/static \
    UPLOAD_DIR=/app/uploads

EXPOSE 8181

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8181/api/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/gameport"]