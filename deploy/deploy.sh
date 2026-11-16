#!/usr/bin/env bash
# =============================================================================
# deploy.sh — pasang artifact backend GamePort ke produksi
#
#   ./deploy.sh <folder-artifact>
#
# Artifact dibuat oleh deploy/package.sh (dipakai GitHub Actions) dan berisi:
#   gameport                 binary linux/amd64
#   gameport-scraper         binary scrape konten baru (draf, dijalankan timer)
#   seed.sql                 skema idempoten
#   static/admin.*           panel admin (backend masih melayani UI admin)
#   deploy/deploy.sh         script ini
#   deploy/autodeploy.sh     poller GitHub Release
#   deploy/scrape.sh         pembungkus scraper (baca .env)
#   Caddyfile.domain         referensi config Caddy (disalin, tidak dipasang)
#   .env.example             referensi env (TIDAK menyentuh .env asli)
#   version.txt              commit SHA
#
# Frontend publik TIDAK ada di sini — dilayani Vercel dari folder frontend/.
#
# .env dan uploads/ TIDAK pernah disentuh. Keduanya milik server.
#
# Alur:
#   1. cek artifact + .env
#   2. cadangkan binary/static lama
#   3. hentikan supervisor
#   4. pasang binary, admin, seed
#   5. nyalakan postgres → terapkan skema → nyalakan app
#   6. verifikasi; kalau gagal, rollback otomatis
#   7. simpan script deploy + referensi hanya setelah deploy sehat
# =============================================================================
set -uo pipefail

SRC=${1:-}
DEP=/opt/data/workspace/deployed/game-port
MANAGER=/opt/data/gameport/manager.sh
PGBIN=/opt/data/pylibs/embedded_postgres/pginstall/bin
PGSOCK=/opt/data/gameport/run
export PATH=$PATH:/opt/data/.local/bin

say()  { printf '\033[1m%s\033[0m\n' "$*"; }
ok()   { printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$*"; }
warn() { printf '  \033[33m!\033[0m %s\n' "$*"; }

if [ -z "$SRC" ] || [ ! -d "$SRC" ]; then
  bad "pakai: $0 <folder-artifact>"
  bad "artifact dibuat oleh deploy/package.sh atau diunduh deploy/autodeploy.sh"
  exit 1
fi
SRC=$(cd "$SRC" && pwd)

# ── 0. preflight ────────────────────────────────────────────────────────────
say "0. periksa artifact"
for f in gameport seed.sql static/admin.html static/admin.css static/admin.js; do
  if [ ! -f "$SRC/$f" ]; then
    bad "artifact tidak lengkap: $f"
    exit 1
  fi
done
if [ ! -f "$DEP/.env" ]; then
  bad "$DEP/.env tidak ada"
  bad "buat dulu dari .env.example; script ini tidak pernah membuat/menimpa .env"
  exit 1
fi
if ! grep -qE '^(export[[:space:]]+)?(ADMIN_PASSWORD|ADMIN_PASSWORD_HASH)=.+' "$DEP/.env"; then
  bad ".env belum berisi ADMIN_PASSWORD atau ADMIN_PASSWORD_HASH"
  exit 1
fi

mkdir -p "$DEP" "$DEP/static" "$DEP/uploads"

if ! grep -qE '^(export[[:space:]]+)?CORS_ORIGINS=' "$DEP/.env"; then
  warn "CORS_ORIGINS tidak diset — backend memakai wildcard"
fi

VERSI=$(tr -d '[:space:]' < "$SRC/version.txt" 2>/dev/null || true)
[ -n "$VERSI" ] || VERSI="unknown"
ok "versi ${VERSI:0:12}"
ok "binary $(du -h "$SRC/gameport" | cut -f1)"
ok ".env dipertahankan"

# ── 1. cadangkan versi lama ─────────────────────────────────────────────────
say "1. cadangkan versi lama"
if [ -f "$DEP/gameport" ]; then
  cp -p "$DEP/gameport" "$DEP/gameport.prev" \
    && ok "binary sebelumnya" || warn "gagal mencadangkan binary"
fi
if [ -d "$DEP/static" ]; then
  rm -rf "$DEP/static.prev"
  cp -a "$DEP/static" "$DEP/static.prev" \
    && ok "static sebelumnya" || warn "gagal mencadangkan static"
fi

rollback() {
  bad "deploy gagal — rollback ke versi sebelumnya"
  "$MANAGER" stop >/dev/null 2>&1

  if [ -f "$DEP/gameport.prev" ]; then
    install -m 0755 "$DEP/gameport.prev" "$DEP/gameport" \
      && ok "binary lama dipulihkan" || bad "binary lama gagal dipulihkan"
  else
    bad "tidak ada binary cadangan"
  fi

  if [ -d "$DEP/static.prev" ]; then
    rm -rf "$DEP/static"
    cp -a "$DEP/static.prev" "$DEP/static" \
      && ok "static lama dipulihkan" || bad "static lama gagal dipulihkan"
  fi

  if "$MANAGER" start >/dev/null 2>&1; then
    H=$(curl -sS --max-time 5 -o /dev/null -w '%{http_code}' \
          "http://127.0.0.1:8181/api/health" 2>/dev/null) || H="000"
    [ "$H" = "200" ] && ok "versi lama sehat" || bad "versi lama juga tidak sehat (HTTP $H)"
  else
    bad "versi lama gagal dinyalakan"
  fi
  exit 1
}

# ── 2. hentikan ─────────────────────────────────────────────────────────────
say "2. hentikan supervisor"
if ! "$MANAGER" stop >/dev/null 2>&1; then
  bad "gagal menghentikan — proses mungkin masih memegang port 8181"
  exit 1
fi
sleep 1

# ── 3. pasang binary + admin + seed ─────────────────────────────────────────
say "3. pasang ke produksi"
if ! install -m 0755 "$SRC/gameport" "$DEP/gameport"; then
  bad "binary gagal dipasang"
  rollback
fi
ok "binary"

if [ -f "$SRC/gameport-scraper" ]; then
  if install -m 0755 "$SRC/gameport-scraper" "$DEP/gameport-scraper"; then
    ok "gameport-scraper"
  else
    warn "gameport-scraper gagal dipasang (scrape terjadwal dilewati)"
  fi
else
  warn "artifact tanpa gameport-scraper — lewati binary scraper"
fi

rm -rf "$DEP/static"
mkdir -p "$DEP/static"
if ! cp -a "$SRC/static/." "$DEP/static/"; then
  bad "static/admin.* gagal dipasang"
  rollback
fi
ok "static/admin.*"

if ! cp "$SRC/seed.sql" "$DEP/seed.sql"; then
  bad "seed.sql gagal disalin"
  rollback
fi
ok "seed.sql"

# ── 4. nyalakan database lebih dulu ─────────────────────────────────────────
say "4. nyalakan database"
if ! "$PGBIN/pg_ctl" -D /opt/data/gameport/pgdata status >/dev/null 2>&1; then
  "$PGBIN/pg_ctl" -D /opt/data/gameport/pgdata \
    -l /opt/data/gameport/logs/postgres.log \
    -o "-k $PGSOCK -p 5432 -c listen_addresses=''" -w start >/dev/null 2>&1
fi
if "$PGBIN/pg_ctl" -D /opt/data/gameport/pgdata status >/dev/null 2>&1; then
  ok "postgres siap"
else
  bad "postgres gagal start"
  rollback
fi

# ── 5. migrasi DB (idempoten, butuh postgres hidup) ─────────────────────────
say "5. terapkan skema (idempoten)"
SEED_FAIL=0
if "$PGBIN/psql" -v ON_ERROR_STOP=1 -h "$PGSOCK" -p 5432 -U gameport -d gamedb \
     -f "$DEP/seed.sql" >/tmp/gp_seed.log 2>&1; then
  grep -cE '^(ALTER|CREATE)' /tmp/gp_seed.log | xargs -I{} echo "  {} pernyataan DDL diterapkan"
  ok "skema sinkron"
else
  SEED_FAIL=1
  bad "skema gagal — lihat /tmp/gp_seed.log"
  tail -5 /tmp/gp_seed.log | sed 's/^/    /'
fi
rm -f /tmp/gp_seed.log

# ── 6. nyalakan aplikasi ────────────────────────────────────────────────────
say "6. nyalakan aplikasi"
if ! "$MANAGER" start >/dev/null 2>&1; then
  bad "gagal start"
  rollback
fi
ok "supervisor jalan"

# ── 7. verifikasi ───────────────────────────────────────────────────────────
say "7. verifikasi"
B=http://127.0.0.1:8181
AP=$(grep -E '^(export[[:space:]]+)?ADMIN_PATH=' "$DEP/.env" 2>/dev/null | tail -1 | cut -d= -f2- | tr -d "\"' \r")
AP=${AP:-/panel-x7k2}
AP=${AP%/}
[ -n "$AP" ] || AP=/panel-x7k2
FAIL=0

cek() { curl -sS --max-time 8 -o /dev/null -w '%{http_code}' "$2"; }
assert200() {
  local code
  code=$(cek "$1" "$2") || code="000"
  [ -n "$code" ] || code="000"
  [ "$code" = "200" ] && ok "$1 $2" || { bad "$1 $3 (HTTP $code)"; FAIL=1; }
}

assert200 "health"      "$B/api/health"       "/api/health"
assert200 "health-v1"   "$B/api/v1/health"    "/api/v1/health"
assert200 "statistik"   "$B/api/v1/statistik" "/api/v1/statistik"
assert200 "admin"       "$B$AP"               "$AP"
assert200 "admin-css"   "$B/static/admin.css" "/static/admin.css"
assert200 "admin-js"    "$B/static/admin.js"  "/static/admin.js"

# ambil satu slug nyata untuk menguji halaman detail API
SLUG=$("$PGBIN/psql" -h "$PGSOCK" -p 5432 -U gameport -d gamedb -tAc \
       "select slug from ports where published order by id limit 1" 2>/dev/null | tr -d ' ')
if [ -n "$SLUG" ]; then
  assert200 "api-detail" "$B/api/v1/ports/$SLUG" "/api/v1/ports/$SLUG"
  assert200 "api-legacy" "$B/api/ports/$SLUG"    "/api/ports/$SLUG"
else
  echo "  (belum ada port terbit — uji detail dilewati)"
fi

[ "$SEED_FAIL" = "0" ] || { bad "skema dilaporkan gagal"; FAIL=1; }

if [ "$FAIL" != "0" ]; then
  rollback
fi

# ── 8. simpan script + referensi (hanya setelah deploy sehat) ───────────────
say "8. simpan script + referensi"
if [ -f "$SRC/Caddyfile.domain" ]; then
  cp "$SRC/Caddyfile.domain" "$DEP/Caddyfile.domain" \
    && ok "Caddyfile.domain (referensi)" || warn "Caddyfile.domain gagal disalin"
fi
if [ -f "$SRC/.env.example" ]; then
  cp "$SRC/.env.example" "$DEP/.env.example" \
    && ok ".env.example (referensi)" || warn ".env.example gagal disalin"
fi

mkdir -p "$DEP/deploy"
for f in deploy.sh autodeploy.sh scrape.sh \
         gameport-autodeploy.service gameport-autodeploy.timer \
         gameport-scraper.service gameport-scraper.timer; do
  [ -f "$SRC/deploy/$f" ] || continue
  tmp="$DEP/deploy/.$f.new"
  cp "$SRC/deploy/$f" "$tmp" || { warn "$f gagal disalin"; continue; }
  chmod 0644 "$tmp"
  case "$f" in
    *.sh) chmod 0755 "$tmp" ;;
  esac
  mv -f "$tmp" "$DEP/deploy/$f" && ok "deploy/$f"
done

# ── 9. selesai ──────────────────────────────────────────────────────────────
echo
say "selesai — versi ${VERSI:0:12}"
"$MANAGER" status | sed -n '1,7p'
echo
echo "Frontend publik dilayani Vercel; VPS ini hanya backend + panel admin."
echo "Kalau Caddyfile.domain berubah: salin ke host lalu reload Caddy."
