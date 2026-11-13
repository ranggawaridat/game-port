#!/usr/bin/env bash
# =============================================================================
# GamePort — manager (dijalankan DI DALAM container Hermes)
#
#   ./manager.sh start     nyalakan postgres + gameport
#   ./manager.sh stop      matikan keduanya
#   ./manager.sh restart   stop lalu start
#   ./manager.sh status    cek kondisi + uji HTTP
#   ./manager.sh logs      ikuti log gameport
#
# PostgreSQL hanya lewat UNIX SOCKET — tidak ada port TCP yang dibuka.
# GamePort listen di 0.0.0.0:8181 supaya bisa dijangkau host.
# =============================================================================
set -uo pipefail

PGBIN=/opt/data/pylibs/embedded_postgres/pginstall/bin
PGDATA=/opt/data/gameport/pgdata
PGSOCK=/opt/data/gameport/run
PGPORT=5432
APP=/opt/data/workspace/deployed/game-port/gameport
APPDIR=/opt/data/workspace/deployed/game-port
ENVFILE=$APPDIR/.env
LOGDIR=/opt/data/gameport/logs
PGLOG=$LOGDIR/postgres.log
APPLOG=$LOGDIR/gameport.log
SUPPID=$LOGDIR/supervisor.pid

mkdir -p "$LOGDIR" "$PGSOCK" "$APPDIR/uploads"

log() { printf '%s %s\n' "$(date '+%H:%M:%S')" "$*"; }

pg_running() {
  [ -f "$PGDATA/postmaster.pid" ] && \
    "$PGBIN/pg_ctl" -D "$PGDATA" status >/dev/null 2>&1
}

app_running() {
  [ -f "$SUPPID" ] && kill -0 "$(cat "$SUPPID" 2>/dev/null)" 2>/dev/null
}

start_pg() {
  if pg_running; then log "postgres: sudah jalan"; return 0; fi
  log "postgres: menyalakan…"
  "$PGBIN/pg_ctl" -D "$PGDATA" -l "$PGLOG" \
    -o "-k $PGSOCK -p $PGPORT -c listen_addresses=''" -w start >/dev/null 2>&1
  if pg_running; then log "postgres: siap (socket $PGSOCK/.s.PGSQL.$PGPORT)"; else
    log "postgres: GAGAL — lihat $PGLOG"; tail -15 "$PGLOG"; return 1; fi
}

start_app() {
  if app_running; then log "gameport: sudah jalan (supervisor pid $(cat "$SUPPID"))"; return 0; fi
  # Jangan start kalau port masih dipegang proses lain
  if (exec 3<>/dev/tcp/127.0.0.1/8181) 2>/dev/null; then
    log "gameport: port 8181 masih dipakai — hentikan dulu"
    return 1
  fi
  log "gameport: menyalakan…"
  # Supervisor: jalankan ulang otomatis bila proses mati
  (
    set -a; . "$ENVFILE"; set +a
    cd "$APPDIR"
    while true; do
      ./gameport >>"$APPLOG" 2>&1
      code=$?
      printf '%s gameport berhenti (exit %s), ulang 3 detik…\n' "$(date '+%H:%M:%S')" "$code" >>"$APPLOG"
      sleep 3
    done
  ) >/dev/null 2>&1 &
  echo $! > "$SUPPID"
  # tunggu siap
  for _ in $(seq 1 30); do
    if curl -fsS --max-time 2 "http://127.0.0.1:${APP_PORT:-8181}/api/health" >/dev/null 2>&1; then
      log "gameport: siap di :8181"; return 0
    fi
    sleep 0.5
  done
  log "gameport: tidak merespons — lihat $APPLOG"; tail -20 "$APPLOG"; return 1
}

stop_app() {
  if [ -f "$SUPPID" ]; then
    pid=$(cat "$SUPPID" 2>/dev/null)
    if [ -n "${pid:-}" ] && kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid" 2>/dev/null
      sleep 1
      kill -KILL "$pid" 2>/dev/null
    fi
    rm -f "$SUPPID"
  fi
  # Sapu proses gameport sampai habis.
  # PENTING: cocokkan lewat /proc/PID/exe (path asli), bukan cmdline —
  # supervisor menjalankan "./gameport" sehingga cmdline tidak memuat path penuh.
  for attempt in 1 2 3 4 5 6 7 8; do
    found=0
    for p in $(ls /proc | grep -E '^[0-9]+$'); do
      [ -d "/proc/$p" ] || continue
      [ "$p" = "$$" ] && continue
      exe=$(readlink "/proc/$p/exe" 2>/dev/null) || continue
      if [ "$exe" = "$APP" ]; then
        found=1
        kill -9 "$p" 2>/dev/null
      fi
    done
    [ "$found" = "0" ] && break
    sleep 0.5
  done
  # Tunggu port 8181 benar-benar lepas
  for _ in $(seq 1 20); do
    if ! (exec 3<>/dev/tcp/127.0.0.1/8181) 2>/dev/null; then return 0; fi
    sleep 0.3
  done
  log "PERINGATAN: port 8181 masih dipakai proses lain"
  return 1
}

stop_pg() {
  if pg_running; then
    log "postgres: mematikan…"
    "$PGBIN/pg_ctl" -D "$PGDATA" -m fast -w stop >/dev/null 2>&1
    log "postgres: berhenti"
  else
    log "postgres: sudah mati"
  fi
}

status() {
  echo "── PostgreSQL ──"
  if pg_running; then
    echo "  status : JALAN (unix socket, TCP tertutup)"
    "$PGBIN/psql" -h "$PGSOCK" -p "$PGPORT" -U gameport -d gamedb \
      -tAc "select '  versi  : '||version()" 2>/dev/null | cut -c1-60
    "$PGBIN/psql" -h "$PGSOCK" -p "$PGPORT" -U gameport -d gamedb \
      -tAc "select '  port   : '||count(*) from ports" 2>/dev/null
  else
    echo "  status : MATI"
  fi
  echo "── GamePort ──"
  if app_running; then
    echo "  supervisor: JALAN (pid $(cat "$SUPPID"))"
  else
    echo "  supervisor: MATI"
  fi
  code=$(curl -sS --max-time 3 -o /dev/null -w '%{http_code}' \
         "http://127.0.0.1:8181/api/health" 2>/dev/null || echo "000")
  echo "  HTTP /api/health: $code"
  if [ "$code" = "200" ]; then
    curl -sS --max-time 3 "http://127.0.0.1:8181/api/health" 2>/dev/null; echo
  fi
  # port 8181 listen di 0.0.0.0?
  if grep -qE ':\s*(1FF5)' /proc/net/tcp 2>/dev/null; then
    echo "  8181: listen ✅ (bisa dijangkau host lewat 172.18.0.3)"
  fi
  echo "── URL ──"
  echo "  lokal  : http://127.0.0.1:8181"
  echo "  dari host: http://172.18.0.3:8181"
  echo "  admin  : $(grep -E '^ADMIN_PATH=' "$ENVFILE" 2>/dev/null | cut -d= -f2)"
}

case "${1:-status}" in
  start)   start_pg && start_app ;;
  stop)    stop_app; stop_pg ;;
  restart) stop_app; stop_pg; sleep 1; start_pg && start_app ;;
  status)  status ;;
  logs)    tail -f "$APPLOG" ;;
  *) echo "pakai: $0 {start|stop|restart|status|logs}"; exit 1 ;;
esac
