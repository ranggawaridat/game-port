#!/usr/bin/env bash
# =============================================================================
# deploy.sh — DEV → PROD untuk GamePort
#
#   ./deploy/deploy.sh
#
# Alur:
#   1. build binary statis dari source di repo
#   2. hentikan supervisor (wajib: binary yang jalan tidak bisa ditimpa)
#   3. salin binary + static + seed.sql ke folder deployed
#   4. nyalakan lagi
#   5. verifikasi: health, satu halaman publik, satu halaman detail
#
# Yang TIDAK dilakukan script ini: git commit/push, dan migrasi DB.
# Migrasi DB dijalankan manual: psql -f db/seed.sql (idempoten).
# =============================================================================
set -uo pipefail

DEV=/opt/data/workspace/projects/game-port
DEP=/opt/data/workspace/deployed/game-port
MANAGER=/opt/data/gameport/manager.sh
PGBIN=/opt/data/pylibs/embedded_postgres/pginstall/bin
PGSOCK=/opt/data/gameport/run
export PATH=$PATH:/opt/data/.local/bin

say() { printf '\033[1m%s\033[0m\n' "$*"; }
ok()  { printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad() { printf '  \033[31m✗\033[0m %s\n' "$*"; }

cd "$DEV" || { bad "folder dev tidak ada"; exit 1; }

# ── 1. build ────────────────────────────────────────────────────────────────
say "1. build"
if ! CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
     go build -trimpath -ldflags="-s -w" -o /tmp/gp_deploy ./cmd/server; then
  bad "build gagal — tidak ada yang diubah"; exit 1
fi
ok "binary $(du -h /tmp/gp_deploy | cut -f1)"

# ── 2. uji cepat sebelum menyentuh produksi ──────────────────────────────────
say "2. go vet"
go vet ./... 2>&1 | head -5
ok "vet selesai"

# ── 3. hentikan ─────────────────────────────────────────────────────────────
say "3. hentikan supervisor"
if ! "$MANAGER" stop >/dev/null 2>&1; then
  bad "gagal menghentikan — proses mungkin masih memegang port 8181"
fi
sleep 1

# ── 4. salin ────────────────────────────────────────────────────────────────
say "4. salin ke produksi"
cp /tmp/gp_deploy "$DEP/gameport" && chmod +x "$DEP/gameport" \
  && ok "binary" || { bad "binary gagal disalin"; rm -f /tmp/gp_deploy; exit 1; }
cp "$DEV"/static/* "$DEP/static/" && ok "static/" || bad "static gagal"
cp "$DEV"/db/seed.sql "$DEP/seed.sql" 2>/dev/null && ok "seed.sql"
rm -f /tmp/gp_deploy

# ── 5. nyalakan database lebih dulu ─────────────────────────────────────────
say "5. nyalakan database"
if ! "$PGBIN/pg_ctl" -D /opt/data/gameport/pgdata status >/dev/null 2>&1; then
  "$PGBIN/pg_ctl" -D /opt/data/gameport/pgdata \
    -l /opt/data/gameport/logs/postgres.log \
    -o "-k $PGSOCK -p 5432 -c listen_addresses=''" -w start >/dev/null 2>&1
fi
if "$PGBIN/pg_ctl" -D /opt/data/gameport/pgdata status >/dev/null 2>&1; then
  ok "postgres siap"
else
  bad "postgres gagal start"; exit 1
fi

# ── 6. migrasi DB (idempoten, butuh postgres hidup) ─────────────────────────
say "6. terapkan skema (idempoten)"
if "$PGBIN/psql" -h "$PGSOCK" -p 5432 -U gameport -d gamedb -f "$DEP/seed.sql" >/tmp/gp_seed.log 2>&1; then
  grep -cE '^(ALTER|CREATE)' /tmp/gp_seed.log | xargs -I{} echo "  {} pernyataan DDL diterapkan"
  ok "skema sinkron"
else
  bad "skema gagal — lihat /tmp/gp_seed.log"; tail -5 /tmp/gp_seed.log | sed 's/^/    /'
fi
rm -f /tmp/gp_seed.log

# ── 7. nyalakan aplikasi ────────────────────────────────────────────────────
say "7. nyalakan aplikasi"
"$MANAGER" start >/dev/null 2>&1 || { bad "gagal start"; exit 1; }
ok "supervisor jalan"

# ── 8. verifikasi ───────────────────────────────────────────────────────────
say "8. verifikasi"
B=http://127.0.0.1:8181
cek() { curl -sS --max-time 8 -o /dev/null -w '%{http_code}' "$2"; }
H=$(cek health "$B/api/health")
[ "$H" = "200" ] && ok "/api/health 200" || bad "/api/health $H"

R=$(cek root "$B/")
[ "$R" = "200" ] && ok "halaman publik 200" || bad "halaman publik $R"

# ambil satu slug nyata untuk menguji halaman detail
SLUG=$("$PGBIN/psql" -h "$PGSOCK" -p 5432 -U gameport -d gamedb -tAc \
       "select slug from ports where published order by id limit 1" 2>/dev/null | tr -d ' ')
if [ -n "$SLUG" ]; then
  D=$(cek detail "$B/port/$SLUG")
  [ "$D" = "200" ] && ok "/port/$SLUG 200" || bad "/port/$SLUG $D"
  A=$(cek api "$B/api/ports/$SLUG")
  [ "$A" = "200" ] && ok "/api/ports/$SLUG 200" || bad "/api/ports/$SLUG $A"
else
  echo "  (belum ada port terbit — uji detail dilewati)"
fi

S=$(cek css "$B/static/app.js")
[ "$S" = "200" ] && ok "static/app.js 200" || bad "static/app.js $S"

echo
say "selesai"
"$MANAGER" status | sed -n '1,7p'
echo
echo "Jangan lupa: git add -A && git commit && git push"
