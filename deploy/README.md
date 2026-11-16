# Deploy backend GamePort (artifact + auto-deploy)

Frontend publik ada di Vercel. VPS hanya menjalankan backend Go + panel admin.
Server **tidak perlu git clone** dan **tidak perlu Go**: GitHub Actions yang
membangun binary, server mengunduh artifact dari GitHub Release lalu
menjalankannya.

```
push ke main
   │
   ▼
GitHub Actions  ── build binary linux/amd64 ──▶ GitHub Release (latest)
                                                     │
                                     poll tiap 2 menit│  (systemd timer)
                                                     ▼
                            autodeploy.sh ──▶ deploy.sh ──▶ manager.sh
                            (unduh + verifikasi)   (pasang + tes)   (PG + app)
```

`.env` dan `uploads/` tetap di server dan **tidak pernah** ikut artifact.

---

## 1. File yang terlibat

| File | Jalan di | Fungsi |
|---|---|---|
| `.github/workflows/deploy-backend.yml` | GitHub | build + publish release saat push ke `main` |
| `deploy/package.sh` | GitHub Actions / lokal | bikin `gameport-backend-linux-amd64.tar.gz` |
| `deploy/autodeploy.sh` | VPS (container) | poll release, unduh, verifikasi, panggil `deploy.sh` |
| `deploy/deploy.sh` | VPS (container) | pasang binary/admin/seed, tes, rollback |
| `deploy/manager.sh` | VPS (container) | supervisor PostgreSQL + aplikasi |
| `deploy/gameport-autodeploy.service` | VPS (host) | unit oneshot pemanggil `docker exec` |
| `deploy/gameport-autodeploy.timer` | VPS (host) | timer tiap 2 menit |
| `deploy/scrape.sh` | VPS (container) | jalankan `gameport-scraper` dengan DSN dari `.env` |
| `deploy/gameport-scraper.service` | VPS (host) | unit oneshot scrape konten baru (draf) |
| `deploy/gameport-scraper.timer` | VPS (host) | timer harian scrape |

Path penting (di dalam container Hermes):

```
/opt/data/workspace/deployed/game-port/          ← produksi
├── gameport                                     binary
├── gameport.prev                                binary sebelum deploy terakhir
├── gameport-scraper                             binary scrape terjadwal
├── seed.sql
├── static/                                      admin.html, admin.css, admin.js
├── deploy/                                      script + unit (ikut ter-update)
├── Caddyfile.domain                             referensi untuk host
├── .env.example                                 referensi, BUKAN .env
├── .env                                         milik server, tidak disentuh
└── uploads/                                     gambar lama (read-only)
```

---

## 2. `.env` — bagaimana caranya?

`.env` **hanya ada di server**. Isinya tidak pernah masuk Git dan tidak pernah
masuk artifact. `deploy/deploy.sh` hanya memeriksa keberadaannya; file itu
tidak dibuat, disalin, atau ditimpa.

Buat sekali di container:

```bash
docker exec -it -u hermes alpha-hermes bash
cd /opt/data/workspace/deployed/game-port

cat > .env <<'EOF'
# PostgreSQL produksi: unix socket, tanpa TCP.
# Kutip nilai yang mengandung & atau ? karena file ini di-source oleh bash.
DATABASE_URL='postgres://gameport:GANTI_PASSWORD_KUAT@/gamedb?host=/opt/data/gameport/run&sslmode=disable'

APP_HOST=0.0.0.0
APP_PORT=8181
STATIC_DIR=./static
# Hanya untuk menyajikan gambar lama dari fitur unggah yang sudah dihapus.
UPLOAD_DIR=./uploads

# Origin frontend Vercel + panel admin.
CORS_ORIGINS=https://gameport.web.id,https://www.gameport.web.id,https://vps.gameport.web.id

# Wajib: password admin (atau ADMIN_PASSWORD_HASH).
ADMIN_PASSWORD=GANTI_PASSWORD_ADMIN
# Ganti dengan path rahasia milikmu; jangan pakai /admin.
ADMIN_PATH=/panel-x7k2

SAFELINK_ENABLED=false
SAFELINK_WAIT=10
SAFELINK_TTL=300
EOF

chmod 600 .env
exit
```

Catatan `DATABASE_URL`: kalau password mengandung karakter URL (`:`, `@`, `/`,
`?`, `#`, `&`, `%`), percent-encode dulu (mis. `@` → `%40`). Nilai yang
mengandung `&` juga wajib dikutip seperti contoh di atas.

Kalau `.env` lama sudah ada dan sudah jalan, **jangan diganti**. Cukup
pastikan `CORS_ORIGINS` sesuai arsitektur baru:

- `CORS_ORIGINS` harus memuat origin Vercel (`https://gameport.web.id`,
  `https://www.gameport.web.id`) dan origin admin (`https://vps.gameport.web.id`).

Setelah mengubah `.env`, restart app (tidak perlu deploy ulang):

```bash
docker exec -u hermes alpha-hermes /opt/data/gameport/manager.sh restart
```

`manager.sh` yang meng-source `.env` dan menjalankan binary. Binary Go tidak
punya dotenv loader, jadi perubahan `.env` baru terbaca setelah restart.

---

## 3. Bootstrap sekali di VPS

Prasyarat di container: `curl`, `tar`, `sha256sum`, `flock`, `mktemp`
(semuanya standar di image Hermes). Tidak perlu `git` atau `go`.

### 3.1 Pastikan release pertama sudah ada

Push ke `main` (atau jalankan workflow **Build & release backend** manual dari
tab Actions). Workflow menghasilkan release `deploy-<nomor>` dan menandainya
sebagai latest.

Cek dari VPS:

```bash
curl -fsSL https://github.com/ranggawaridat/game-port/releases/latest/download/version.txt
```

Harus keluar commit SHA.

### 3.2 Pasang bootstrap `autodeploy.sh`

Ini satu-satunya unduhan manual. Setelah release pertama jalan, script dan
unit ikut ter-update otomatis dari artifact.

```bash
docker exec -u hermes alpha-hermes bash -lc '
  mkdir -p /opt/data/workspace/deployed/game-port/deploy
  curl -fsSL https://raw.githubusercontent.com/ranggawaridat/game-port/main/deploy/autodeploy.sh \
    -o /opt/data/workspace/deployed/game-port/deploy/autodeploy.sh
  chmod +x /opt/data/workspace/deployed/game-port/deploy/autodeploy.sh
'
```

### 3.3 Deploy pertama

```bash
docker exec -u hermes alpha-hermes \
  /opt/data/workspace/deployed/game-port/deploy/autodeploy.sh

docker exec -u hermes alpha-hermes \
  tail -n 80 /opt/data/gameport/autodeploy/autodeploy.log
```

`deploy.sh` akan memasang binary, admin static, seed, script deploy, dan unit
systemd ke folder deployed. `.env` dan `uploads/` tidak disentuh.

### 3.4 Pasang timer di host

Unit dan timer sudah ikut terpasang di folder deployed oleh langkah 3.3.

```bash
sudo cp /opt/alpha/hermes/workspace/deployed/game-port/deploy/gameport-autodeploy.service \
        /etc/systemd/system/
sudo cp /opt/alpha/hermes/workspace/deployed/game-port/deploy/gameport-autodeploy.timer \
        /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now gameport-autodeploy.timer

systemctl list-timers gameport-autodeploy.timer
```

Kalau nama container bukan `alpha-hermes`, edit `ExecStart` di unit.

### 3.5 Pasang timer scraper (opsional)

Scraper mengisi katalog dari beberapa situs port Android. Semua hasilnya masuk
sebagai **draf** (`published=false`) dan baru tayang setelah ditinjau di panel
admin. Timer ini terpisah dari auto-deploy dan tidak wajib dipasang.

Unit dan script sudah ikut terpasang di folder deployed oleh langkah 3.3.

```bash
sudo cp /opt/alpha/hermes/workspace/deployed/game-port/deploy/gameport-scraper.service \
        /etc/systemd/system/
sudo cp /opt/alpha/hermes/workspace/deployed/game-port/deploy/gameport-scraper.timer \
        /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now gameport-scraper.timer

systemctl list-timers gameport-scraper.timer
```

Jadwal default: sekali sehari pukul 03:30 waktu host, dengan
`RandomizedDelaySec=30min` agar beban keluar tidak menumpuk di detik yang sama.
Ubah `OnCalendar` di unit timer kalau perlu jam lain.

Uji manual tanpa menunggu jadwal:

```bash
# lewat systemd (sama seperti jadwal asli, log ke journal)
sudo systemctl start gameport-scraper.service
journalctl -u gameport-scraper.service -n 80 --no-pager

# langsung di container, dengan flag tambahan
docker exec -u hermes alpha-hermes bash -lc '
  cd /opt/data/workspace/deployed/game-port
  ./deploy/scrape.sh -dry-run          # tidak menulis apa pun
  ./deploy/scrape.sh -only droidports  # satu sumber saja
  ./deploy/scrape.sh -verbose
'
```

Sumber diatur di tabel `scrape_sources` (kolom `enabled`, `max_items`, dan
`kind`: `rss`, `playported`, atau `markdown`). Menambah situs yang sudah
didukung salah satu `kind` cukup dengan satu `INSERT` — tidak perlu build
ulang. Scraper mematuhi robots.txt dan `Crawl-delay` per host, memakai
User-Agent yang jelas, dan tidak menulis `download_url` (draf hanya menunjuk
`source_url`).

Karena isinya selalu draf, kegagalan scrape tidak memengaruhi situs. Kalau
timer dimatikan, katalog tetap jalan seperti biasa.

### 3.6 Pasang Caddy

Caddy berjalan di host, bukan di container. `Caddyfile.domain` ikut
terpasang di folder deployed setiap deploy, tetapi **tidak** otomatis
mengganti config host.

```bash
sudo cp /opt/alpha/hermes/workspace/deployed/game-port/Caddyfile.domain \
        /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Saat ini hanya `api.gameport.web.id` dan `vps.gameport.web.id` yang punya
site block. Apex `gameport.web.id` dan `www` ditangani Vercel — jangan
ditambahkan ke Caddy selama DNS-nya menunjuk Vercel.

---

## 4. Alur update sehari-hari

1. `git push` ke `main`.
2. Kalau yang berubah backend/admin/script deploy, workflow build release baru.
   Perubahan frontend publik saja tidak memicu backend release (Vercel yang
   deploy).
3. Timer di VPS mengecek `releases/latest/download/version.txt` tiap ±2 menit.
4. Kalau SHA berubah: unduh tarball, cocokkan `SHA256SUMS`, ekstrak, jalankan
   `deploy.sh`.
5. `deploy.sh` berhenti sejenak, pasang binary + admin, migrasi skema, nyalakan,
   lalu tes `/api/health`, `/api/v1/health`, `/api/v1/statistik`, halaman admin,
   dan `/static/admin.*`.
6. Kalau tes gagal, binary/static lama dipulihkan otomatis dan versi baru
   **tidak** dicatat. Percobaan ulang ditunda 15 menit agar produksi tidak
   restart terus.

Deploy manual (tanpa menunggu timer):

```bash
docker exec -u hermes alpha-hermes \
  /opt/data/workspace/deployed/game-port/deploy/autodeploy.sh
```

Paksa deploy ulang versi yang sama: hapus penanda versi lalu jalankan lagi.

```bash
docker exec -u hermes alpha-hermes rm -f /opt/data/gameport/autodeploy/version
docker exec -u hermes alpha-hermes \
  /opt/data/workspace/deployed/game-port/deploy/autodeploy.sh
```

---

## 5. Log dan status

```bash
# auto-deploy
docker exec -u hermes alpha-hermes tail -f /opt/data/gameport/autodeploy/autodeploy.log

# aplikasi + postgres
docker exec -u hermes alpha-hermes /opt/data/gameport/manager.sh logs
docker exec -u hermes alpha-hermes /opt/data/gameport/manager.sh status

# systemd (host)
journalctl -u gameport-autodeploy.service -n 50 --no-pager
systemctl status gameport-autodeploy.timer

# scraper (kalau timer dipasang)
journalctl -u gameport-scraper.service -n 80 --no-pager
systemctl status gameport-scraper.timer
```

Log auto-deploy diputar otomatis saat melewati ±5 MB (`autodeploy.log.1`).

---

## 6. Rollback

Otomatis: kalau health/admin/API gagal setelah deploy, `deploy.sh` langsung
memulihkan `gameport.prev` dan `static.prev`.

Manual:

```bash
docker exec -u hermes alpha-hermes bash -lc '
  cd /opt/data/workspace/deployed/game-port
  /opt/data/gameport/manager.sh stop
  cp -p gameport.prev gameport
  rm -rf static && cp -a static.prev static
  /opt/data/gameport/manager.sh start
'
```

Catatan: rollback binary **tidak** membatalkan migrasi skema yang sudah
diterapkan. Karena `seed.sql` harus tetap idempoten, rollback sebaiknya hanya
untuk kegagalan runtime binary, bukan kesalahan skema.

---

## 7. Build artifact manual (opsional)

Dari checkout lokal (butuh Go):

```bash
./deploy/package.sh dist
```

Hasil di `dist/`:

```
gameport-backend-linux-amd64.tar.gz
SHA256SUMS
version.txt
```

Isi tarball: `gameport`, `gameport-scraper`, `seed.sql`, `static/admin.*`,
`deploy/*.sh`, `deploy/gameport-autodeploy.*`, `deploy/gameport-scraper.*`,
`Caddyfile.domain`, `.env.example`, `version.txt`. Tidak ada `.env` dan tidak
ada `frontend/index.html`.

Deploy manual dari artifact:

```bash
docker cp dist/gameport-backend-linux-amd64.tar.gz alpha-hermes:/tmp/gp.tgz
docker exec -u hermes alpha-hermes bash -lc '
  rm -rf /tmp/gp-pkg && mkdir -p /tmp/gp-pkg
  tar -xzf /tmp/gp.tgz -C /tmp/gp-pkg
  /tmp/gp-pkg/deploy/deploy.sh /tmp/gp-pkg
'
```

---

## 8. Troubleshooting

**`version.txt` HTTP 404**

Belum ada release, atau workflow gagal. Cek tab **Actions** di GitHub. Setelah
release ada, timer berikutnya akan mengambilnya.

**Checksum tidak cocok**

Asset release rusak / upload belum selesai. Jalankan ulang workflow, atau
hapus asset lalu upload ulang. `autodeploy.sh` tidak akan memasang artifact
yang checksum-nya salah.

**Deploy gagal berulang**

Lihat `autodeploy.log`. Versi yang sama ditunda 15 menit setelah gagal. Kalau
perlu memperbaiki cepat, push perbaikan (versi baru) atau jalankan `deploy.sh`
manual.

**Caddy 502**

Backend tidak listen di `172.18.0.3:8181`, atau IP container berubah setelah
recreate. Cek:

```bash
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' alpha-hermes
docker exec -u hermes alpha-hermes /opt/data/gameport/manager.sh status
```

Kalau IP berubah, sesuaikan `reverse_proxy` di Caddyfile lalu reload Caddy.

**Admin 404**

Pastikan `ADMIN_PATH` di `.env` sama dengan URL yang dibuka, dan
`static/admin.html` ada di folder deployed. `deploy.sh` menyalin admin static
dari artifact, bukan dari Vercel.

**Repo privat**

Release asset tidak bisa diunduh tanpa token. Untuk repo privat, gunakan
`GAMEPORT_RELEASE_BASE` dengan token, atau ganti arsitektur ke SSH push dari
GitHub Actions. Repo ini publik, jadi tidak perlu.
