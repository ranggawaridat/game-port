#!/usr/bin/env bash
# =============================================================================
# package.sh — bikin artifact backend untuk GitHub Release / deploy manual
#
#   ./deploy/package.sh [folder-output]
#
# Hasil di folder-output (default: ./dist):
#   gameport-backend-linux-amd64.tar.gz   artifact siap unduh
#   SHA256SUMS                             checksum untuk autodeploy
#   version.txt                            commit SHA
#   gameport-backend-linux-amd64/          isi mentah, untuk inspeksi
#
# Artifact berisi binary + seed + admin + script deploy. Frontend publik
# (index.html, app.js, style.css) TIDAK ikut — itu milik Vercel.
# .env asli TIDAK pernah ikut; yang ikut hanya .env.example sebagai referensi.
# =============================================================================
set -euo pipefail

say() { printf '\033[1m%s\033[0m\n' "$*"; }

ROOT=$(cd "$(dirname "$0")/.." && pwd)
CALLER_PWD=$(pwd)
OUT=${1:-$ROOT/dist}
case "$OUT" in
  /*) ;;
  *) OUT=$CALLER_PWD/$OUT ;;
esac
NAME=gameport-backend-linux-amd64
VERSION=${VERSION:-${GITHUB_SHA:-}}

if [ -z "$VERSION" ]; then
  if git -C "$ROOT" rev-parse HEAD >/dev/null 2>&1; then
    VERSION=$(git -C "$ROOT" rev-parse HEAD)
  else
    VERSION=$(date +%Y%m%d%H%M%S)
  fi
fi

STAGE=$OUT/$NAME
rm -rf "$STAGE" "$OUT/$NAME.tar.gz" "$OUT/SHA256SUMS" "$OUT/version.txt"
mkdir -p "$STAGE/static" "$STAGE/deploy"

say "build $VERSION"
cd "$ROOT/backend"
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -buildvcs=false -ldflags="-s -w" \
  -o "$STAGE/gameport" ./cmd/server

cp db/seed.sql "$STAGE/seed.sql"
# Semua aset admin.* (bukan frontend publik) ikut ke backend
cp "$ROOT"/frontend/admin.* "$STAGE/static/"

cp "$ROOT/deploy/deploy.sh"                     "$STAGE/deploy/deploy.sh"
cp "$ROOT/deploy/autodeploy.sh"                 "$STAGE/deploy/autodeploy.sh"
cp "$ROOT/deploy/gameport-autodeploy.service"   "$STAGE/deploy/gameport-autodeploy.service"
cp "$ROOT/deploy/gameport-autodeploy.timer"     "$STAGE/deploy/gameport-autodeploy.timer"
cp "$ROOT/Caddyfile.domain"                     "$STAGE/Caddyfile.domain"
cp "$ROOT/.env.example"                         "$STAGE/.env.example"

printf '%s\n' "$VERSION" > "$STAGE/version.txt"
chmod +x "$STAGE/gameport" "$STAGE/deploy/deploy.sh" "$STAGE/deploy/autodeploy.sh"

tar -C "$STAGE" -czf "$OUT/$NAME.tar.gz" .
( cd "$OUT" && sha256sum "$NAME.tar.gz" > SHA256SUMS )
printf '%s\n' "$VERSION" > "$OUT/version.txt"

say "selesai: $OUT/$NAME.tar.gz ($(du -h "$OUT/$NAME.tar.gz" | cut -f1))"
