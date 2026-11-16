#!/usr/bin/env bash
# =============================================================================
# scrape.sh — jalankan gameport-scraper di dalam container Hermes
#
# Dipanggil systemd timer di host lewat docker exec. `docker exec` tidak
# mewarisi isi .env (file itu hanya dibaca manager.sh), jadi DSN diambil
# langsung dari .env di folder deployed.
#
#   ./deploy/scrape.sh                 # semua sumber aktif
#   ./deploy/scrape.sh -dry-run        # lihat hasil tanpa menulis
#   ./deploy/scrape.sh -only droidports
#
# Hasil scrape selalu draf (published=false) untuk ditinjau di panel admin.
# =============================================================================
set -euo pipefail

DEP=$(cd "$(dirname "$0")/.." && pwd)
BIN=$DEP/gameport-scraper
ENVF=$DEP/.env

if [ ! -x "$BIN" ]; then
  echo "binary scraper tidak ditemukan: $BIN" >&2
  echo "pastikan artifact berisi gameport-scraper (deploy/package.sh)" >&2
  exit 1
fi

# ambil_env membaca satu nilai dari .env tanpa men-source seluruh file.
# Nilai yang mengandung & atau ? tetap aman karena tidak dieksekusi bash.
ambil_env() {
  [ -f "$ENVF" ] || return 0
  grep -E "^(export[[:space:]]+)?$1=" "$ENVF" 2>/dev/null | tail -1 | cut -d= -f2- \
    | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' \
          -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//"
}

if [ -z "${DATABASE_URL:-}" ]; then
  DATABASE_URL=$(ambil_env DATABASE_URL)
fi
export DATABASE_URL=${DATABASE_URL:-}

# Kalau DATABASE_URL kosong, config.LoadDatabaseURL() memakai GAMEDB_*.
for v in GAMEDB_HOST GAMEDB_PORT GAMEDB_NAME GAMEDB_USER GAMEDB_PASSWORD; do
  if [ -z "${!v:-}" ]; then
    val=$(ambil_env "$v")
    if [ -n "$val" ]; then
      export "$v=$val"
    fi
  fi
done

exec "$BIN" "$@"
