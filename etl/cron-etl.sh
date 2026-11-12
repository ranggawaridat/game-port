#!/bin/sh
# =============================================================================
# cron-etl.sh — tugas terjadwal: crawl sumber, muat hanya bila berubah.
#
# Dirancang untuk dijalankan harian. Aman dan murah:
#   - memakai ETag (If-None-Match): sumber tidak berubah -> 304 -> nol kerja
#   - exit 0 saat tidak ada perubahan, jadi tidak memicu alert
#   - exit != 0 hanya bila benar-benar gagal (jaringan/DB/format sumber)
#
# Log: stdout (ditangkap docker logging driver).
# =============================================================================
set -eu

cd /app

echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] mulai cron ETL"

if python etl/etl.py; then
    echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] selesai OK"
else
    code=$?
    # 2 = sumber berubah format / baris terlalu sedikit (DB tidak disentuh)
    if [ "$code" -eq 2 ]; then
        echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] PERINGATAN: sumber tidak valid, DB aman" >&2
    else
        echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] GAGAL dengan kode $code" >&2
    fi
    exit "$code"
fi