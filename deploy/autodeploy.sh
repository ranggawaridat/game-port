#!/usr/bin/env bash
# =============================================================================
# autodeploy.sh — poll GitHub Release, pasang backend baru tanpa git clone
#
# Dipanggil oleh systemd timer di host:
#   docker exec -u hermes alpha-hermes \
#     /opt/data/workspace/deployed/game-port/deploy/autodeploy.sh
#
# Alur:
#   1. ambil version.txt dari release "latest"
#   2. bandingkan dengan versi terakhir yang sukses
#   3. unduh tarball + SHA256SUMS, verifikasi, ekstrak
#   4. jalankan deploy/deploy.sh dari artifact
#   5. simpan versi hanya jika deploy sukses
#
# .env dan uploads/ tidak pernah diunduh/ditimpa.
# Repo harus publik (release asset diunduh tanpa token). Kalau repo privat,
# ganti BASE ke URL yang menyertakan token, atau pakai SSH push dari Actions.
# =============================================================================
set -uo pipefail

REPO=${GAMEPORT_GITHUB_REPO:-ranggawaridat/game-port}
BASE=${GAMEPORT_RELEASE_BASE:-https://github.com/$REPO/releases/latest/download}
ASSET=${GAMEPORT_ASSET:-gameport-backend-linux-amd64.tar.gz}
DEP=${GAMEPORT_DEPLOY_DIR:-/opt/data/workspace/deployed/game-port}
WORK=${GAMEPORT_AUTODEPLOY_DIR:-/opt/data/gameport/autodeploy}
STATE=$WORK/version
RETRY=$WORK/retry
LOG=$WORK/autodeploy.log
RETRY_SECONDS=${GAMEPORT_RETRY_SECONDS:-900}
export PATH=$PATH:/opt/data/.local/bin

mkdir -p "$WORK" || { echo "tidak bisa membuat $WORK" >&2; exit 1; }

# putar log kasar supaya tidak tumbuh tanpa batas
if [ -f "$LOG" ] && [ "$(stat -c%s "$LOG" 2>/dev/null || echo 0)" -gt 5242880 ]; then
  mv -f "$LOG" "$LOG.1"
fi

exec >>"$LOG" 2>&1
log() { printf '%s %s\n' "$(date '+%F %T')" "$*"; }

command -v curl      >/dev/null || { log "curl tidak ada"; exit 1; }
command -v tar       >/dev/null || { log "tar tidak ada"; exit 1; }
command -v sha256sum >/dev/null || { log "sha256sum tidak ada"; exit 1; }
command -v flock     >/dev/null || { log "flock tidak ada"; exit 1; }
command -v mktemp    >/dev/null || { log "mktemp tidak ada"; exit 1; }

# cegah dua deploy berjalan bersamaan
exec 9>"$WORK/lock"
if ! flock -n 9; then
  log "lewati: proses autodeploy lain masih berjalan"
  exit 0
fi

# ── 1. versi terbaru ────────────────────────────────────────────────────────
VFILE=$(mktemp "$WORK/version.XXXXXX") || { log "mktemp gagal"; exit 1; }
CODE=$(curl -sS -L --retry 3 --retry-delay 2 --max-time 20 \
       -o "$VFILE" -w '%{http_code}' "$BASE/version.txt" 2>/dev/null) || CODE="000"
if [ "$CODE" = "404" ]; then
  rm -f "$VFILE"
  log "belum ada release backend (HTTP 404) — lewati"
  exit 0
fi
if [ "$CODE" != "200" ]; then
  rm -f "$VFILE"
  log "gagal mengambil version.txt (HTTP $CODE)"
  exit 1
fi
VERSION=$(tr -d '[:space:]' < "$VFILE")
rm -f "$VFILE"

if [ -z "$VERSION" ]; then
  log "version.txt kosong"
  exit 1
fi
case "$VERSION" in
  *[!A-Za-z0-9._-]*) log "versi tidak valid: $VERSION"; exit 1 ;;
esac

LAST=""
[ -f "$STATE" ] && LAST=$(tr -d '[:space:]' < "$STATE")
if [ "$VERSION" = "$LAST" ]; then
  exit 0
fi

# backoff kalau versi ini pernah gagal, supaya tidak restart produksi terus
if [ -f "$RETRY" ]; then
  RV=$(cut -f1 "$RETRY" 2>/dev/null || true)
  RT=$(cut -f2 "$RETRY" 2>/dev/null || true)
  NOW=$(date +%s)
  if [ "$RV" = "$VERSION" ] && [ -n "$RT" ] && [ $((NOW - RT)) -lt "$RETRY_SECONDS" ]; then
    log "versi $VERSION gagal sebelumnya; coba lagi dalam $((RETRY_SECONDS - (NOW - RT))) detik"
    exit 0
  fi
fi

log "update tersedia: ${LAST:-<belum ada>} -> $VERSION"

# ── 2. unduh + verifikasi artifact ─────────────────────────────────────────
TMP=$(mktemp -d "$WORK/tmp.XXXXXX") || { log "mktemp gagal"; exit 1; }
trap 'rm -rf "$TMP"' EXIT

if ! curl -fsSL --retry 3 --retry-delay 2 --max-time 180 \
     "$BASE/$ASSET" -o "$TMP/$ASSET"; then
  log "gagal mengunduh $ASSET"
  exit 1
fi
if ! curl -fsSL --retry 3 --retry-delay 2 --max-time 30 \
     "$BASE/SHA256SUMS" -o "$TMP/SHA256SUMS"; then
  log "gagal mengunduh SHA256SUMS"
  exit 1
fi

if ! OUT=$(cd "$TMP" && sha256sum -c SHA256SUMS 2>&1); then
  log "checksum tidak cocok: $OUT"
  exit 1
fi

mkdir -p "$TMP/pkg"
if ! tar -xzf "$TMP/$ASSET" -C "$TMP/pkg"; then
  log "gagal mengekstrak artifact"
  exit 1
fi
# jangan bergantung pada umask saat ekstrak
chmod +x "$TMP/pkg/gameport" "$TMP/pkg/deploy/deploy.sh" 2>/dev/null || true
if [ ! -f "$TMP/pkg/gameport" ] || [ ! -f "$TMP/pkg/deploy/deploy.sh" ]; then
  log "artifact tidak lengkap"
  exit 1
fi
if [ -f "$TMP/pkg/version.txt" ] &&
   [ "$(tr -d '[:space:]' < "$TMP/pkg/version.txt")" != "$VERSION" ]; then
  log "version.txt artifact tidak cocok dengan release"
  exit 1
fi

# ── 3. deploy ───────────────────────────────────────────────────────────────
log "menjalankan deploy.sh"
if bash "$TMP/pkg/deploy/deploy.sh" "$TMP/pkg"; then
  printf '%s\n' "$VERSION" > "$STATE"
  rm -f "$RETRY"
  log "deploy sukses: $VERSION"
  exit 0
fi

printf '%s\t%s\n' "$VERSION" "$(date +%s)" > "$RETRY"
log "deploy GAGAL: $VERSION — dicoba lagi setelah $RETRY_SECONDS detik"
exit 1
