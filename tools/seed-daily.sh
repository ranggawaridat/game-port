#!/usr/bin/env bash
# =============================================================================
# seed-daily.sh — segarkan katalog GamePort dari rilis GitHub.
# Dijalankan dari HOST (cron), bukan dari dalam container Hermes.
#
#   /opt/gameport/tools/seed-daily.sh
#
# Yang dilakukan:
#   1. git pull --ff-only  (ambil kode + daftar CANDIDATES terbaru)
#   2. tools/seed-github.py --update  → SQL idempotent
#   3. terapkan ke container gameport-db
#   4. catat hasil ke logs/seed-github.log
#
# Aman dijalankan berulang. Entri yang tautannya sudah kamu ganti ke sfl.gl
# TIDAK akan ditimpa (lihat WHERE di mode --update).
# =============================================================================
set -uo pipefail

APP_DIR="${APP_DIR:-/opt/gameport}"
DB_CONTAINER="${DB_CONTAINER:-gameport-db}"
DB_NAME="${DB_NAME:-gamedb}"
DB_USER="${DB_USER:-gameport}"
LOG="${LOG:-$APP_DIR/logs/seed-github.log}"

mkdir -p "$(dirname "$LOG")"
log() { printf '%s  %s\n' "$(date '+%F %T')" "$*" >>"$LOG"; }

cd "$APP_DIR" || { log "GAGAL: $APP_DIR tidak ada"; exit 1; }

if [ -d .git ]; then
  git pull --ff-only -q 2>>"$LOG" || log "PERINGATAN: git pull gagal — pakai kode lokal"
fi

SQL="$(mktemp /tmp/seed-github-XXXXXX.sql)"
trap 'rm -f "$SQL"' EXIT

if ! python3 tools/seed-github.py --update -o "$SQL" 2>>"$LOG"; then
  log "GAGAL: generator SQL error"; exit 1
fi

if docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" \
     -q -v ON_ERROR_STOP=1 <"$SQL" >>"$LOG" 2>&1; then
  N=$(docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" \
        -tAc "SELECT count(*) FROM ports" 2>/dev/null)
  log "OK — katalog sekarang ${N:-?} port"
else
  log "GAGAL: psql menolak SQL"; exit 1
fi
