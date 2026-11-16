#!/bin/sh
# =============================================================================
# seeder-run.sh — penyegar katalog GamePort dari dalam container.
#
# Bukan cron: prosesnya menjalankan satu putaran saat container menyala
# (SEED_RUN_ON_START), lalu menunggu sampai jam yang diminta (UTC) dan
# mengulanginya setiap hari. Tidur dihitung ulang tiap putaran, jadi
# mengubah SEED_HOUR_UTC cukup dengan restart container.
#
# Variabel (dari docker compose):
#   PGHOST, PGUSER, PGDATABASE, PGPASSWORD  ← koneksi ke service db
#   SEED_HOUR_UTC                            ← jam jalan harian, default 0
#                                              (00:00 UTC = 07:00 WIB)
#   SEED_RUN_ON_START                        ← "false" = jangan jalan saat
#                                              start, tunggu jam harian dulu
# =============================================================================
set -u

: "${PGHOST:=db}"
: "${PGUSER:=gameport}"
: "${PGDATABASE:=gamedb}"
: "${SEED_HOUR_UTC:=0}"
: "${SEED_RUN_ON_START:=true}"

log() { printf '%s  %s\n' "$(date -u '+%F %T UTC')" "$*"; }

# detik sampai jam SEED_HOUR_UTC berikutnya
detik_tunggu() {
  SEED_HOUR_UTC="$SEED_HOUR_UTC" python3 - <<'PY'
import os, time
h = int(os.environ.get("SEED_HOUR_UTC", "0")) % 24
now = time.time()
t = time.gmtime(now)
target = now - (t.tm_hour * 3600 + t.tm_min * 60 + t.tm_sec) + h * 3600
if target <= now:
    target += 86400
print(int(target - now))
PY
}

# satu putaran: tarik GitHub → SQL → terapkan ke database
putaran() {
  log "──── mulai putaran ────"
  if python3 /src/tools/seed-github.py --update -o /tmp/seed.sql; then
    if psql -q -v ON_ERROR_STOP=1 -f /tmp/seed.sql; then
      N=$(psql -tAc "SELECT count(*) FROM ports" 2>/dev/null)
      log "OK — katalog sekarang ${N:-?} port"
    else
      log "GAGAL: psql menolak SQL"
    fi
  else
    log "GAGAL: generator tidak menghasilkan SQL"
  fi
  rm -f /tmp/seed.sql
}

log "seeder siap — jadwal harian ${SEED_HOUR_UTC}:00 UTC"

if [ "$SEED_RUN_ON_START" != "false" ]; then
  log "putaran awal saat container start"
  putaran
fi

while true; do
  s=$(detik_tunggu) || { log "GAGAL menghitung jadwal"; sleep 3600; continue; }
  log "menunggu ${s} detik sampai ${SEED_HOUR_UTC}:00 UTC…"
  sleep "$s"
  putaran
done