# GamePort

Katalog port native Android dari game PC & konsol — source port, dekompilasi,
dan rekompilasi statis yang menjalankan kode game langsung di perangkat,
bukan emulasi.

**Live:** <https://gameport.web.id>

Backend **Go** · database **PostgreSQL** · frontend **vanilla HTML/CSS/JS** ·
panel admin bawaan. Konten diisi **manual** lewat panel admin — tidak ada
scraping, tidak ada crawler.

![GamePort](docs/screenshot.png)

---

## Fitur

**Publik**

- Katalog dengan gambar sampul, versi, ukuran berkas, genre, dan platform
- Pencarian teks, filter kategori/genre/platform, 7 mode pengurutan
- Hitungan tampilan dan unduhan per game
- Tema gelap, responsif, tanpa framework dan tanpa build step

**Admin** — di path rahasia, tidak ditautkan dari halaman publik

- Statistik: total port, tampilan, unduhan, grafik 14 hari, game paling
  dilihat, pencarian teratas
- CRUD lengkap: tambah, ubah, hapus port
- Unggah gambar sampul (maks 5 MB, diverifikasi lewat magic bytes)
- Status draf — konten bisa disimpan tanpa dipublikasikan
- Sesi cookie 12 jam, disimpan di database, dihapus saat logout

---

## Tumpukan

| Bagian | Teknologi |
|---|---|
| Bahasa | Go 1.23 (binary statis, `CGO_ENABLED=0`) |
| Driver DB | `jackc/pgx/v5` (connection pool) |
| Database | PostgreSQL 17/18 |
| Frontend | HTML + CSS + JS murni |
| Reverse proxy | Caddy (HTTPS otomatis) |
| Orkestrasi | Docker Compose, atau systemd |

Tidak ada Node.js, tidak ada bundler, tidak ada dependency frontend.

---

## Struktur

```
backend/
  cmd/server/main.go        entrypoint + graceful shutdown
  internal/
    config/config.go        env, escape kredensial DSN
    db/                     pool pgx, model, query
    api/                    router publik, admin, static, CORS
  db/seed.sql               skema + data referensi (idempoten)
frontend/                   situs publik (Vercel) + admin static
  index.html  app.js  style.css
  admin.html  admin.js  admin.css
  vercel.json
deploy/
  package.sh                build artifact backend (CI)
  autodeploy.sh             poll GitHub Release di VPS
  deploy.sh                 pasang artifact + rollback
  manager.sh                PostgreSQL + app di container
.github/workflows/deploy-backend.yml
docs/screenshot.png
Caddyfile.domain            reverse proxy api. + vps. (apex/www di Vercel)
Caddyfile.ip                reverse proxy untuk akses lewat IP mentah
gameport.service            unit systemd (cara lama, host)
docker-compose.yml          db + backend
Dockerfile                  multi-stage build
```

---

## Mulai cepat

```bash
cp .env.example .env     # isi GAMEDB_PASSWORD dan ADMIN_PASSWORD
docker compose up -d --build
```

Buka <http://127.0.0.1:8181>. Panel admin di `ADMIN_PATH`
(default `/panel-x7k2`).

`ADMIN_PASSWORD` **wajib** diisi — server menolak start kalau kosong.

---

## Deploy

Arsitektur produksi saat ini:

```
gameport.web.id      → Vercel (frontend statis, root directory frontend/)
api.gameport.web.id  → VPS (API Go, safelink, /uploads)
vps.gameport.web.id  → VPS (panel admin)
```

Backend memakai **artifact + auto-deploy**, tanpa git clone dan tanpa Go di
server:

1. Push ke `main` → GitHub Actions menjalankan `deploy/package.sh`.
2. Workflow menerbitkan GitHub Release (`latest`) berisi binary, seed, admin
   static, dan script deploy.
3. Timer di VPS mengecek release tiap ±2 menit, mengunduh, memverifikasi
   checksum, lalu menjalankan `deploy/deploy.sh`.
4. `deploy.sh` memasang binary/admin, migrasi skema, tes health/API/admin, dan
   rollback otomatis kalau gagal.

`.env` dan `uploads/` tetap di server dan tidak pernah ikut artifact.

Panduan lengkap: **[deploy/README.md](deploy/README.md)**.

Perintah cepat:

```bash
# deploy manual, tanpa menunggu timer
docker exec -u hermes alpha-hermes \
  /opt/data/workspace/deployed/game-port/deploy/autodeploy.sh

# status PostgreSQL + aplikasi
docker exec -u hermes alpha-hermes /opt/data/gameport/manager.sh status
```

Catatan lama:

- `README-DEPLOY.md` — cara lama: binary + systemd di host.
- `SETUP-DOMAIN.md` — DNS/HTTPS; sebagian masih menjelaskan setup monolitik.

### Caddy

`Caddyfile.domain` sekarang hanya berisi blok `api.` dan `vps.`. Apex dan `www`
ditangani Vercel — jangan menambahkan blok apex/www selama DNS-nya menunjuk
Vercel.

```bash
sudo cp /opt/alpha/hermes/workspace/deployed/game-port/Caddyfile.domain \
        /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

> **Jangan tulis IP sebagai site address** (`149.x.x.x { ... }`). Caddy akan
> memaksa HTTPS dan membalas **308** redirect ke port 443 — gagal kalau 443
> tertutup atau sertifikat tidak ada. Pakai `:80` untuk akses lewat IP.

---

## API publik

Base: **`https://api.gameport.web.id`**. Path saat ini memakai `/api/v1/...`;
`/api/...` tetap tersedia sebagai alias migrasi.

| Endpoint | Keterangan |
|---|---|
| `GET /api/v1/health` | status koneksi DB + pool |
| `GET /api/v1/statistik` | agregat katalog |
| `GET /api/v1/kategori` | kategori + jumlah |
| `GET /api/v1/status` | status kematangan + jumlah |
| `GET /api/v1/platform` | platform + jumlah |
| `GET /api/v1/genre` | genre + jumlah |
| `GET /api/v1/ports` | daftar port (filter, sort, paginasi) |
| `GET /api/v1/ports/{slug}` | detail + 6 port terkait |
| `GET /api/v1/ports/{slug}/download` | catat unduhan, lalu redirect |

Parameter `/api/v1/ports`:

- `q` — cari di nama, deskripsi, author
- `kategori`, `genre`, `platform` — filter slug
- `own=0|1` — butuh data game asli atau tidak
- `sort` — `kategori` `nama` `nama-desc` `status` `populer` `terbaru` `download`
- `limit` 1–200 (default 24), `offset`

```bash
curl 'https://api.gameport.web.id/api/v1/ports?q=half-life'
curl 'https://api.gameport.web.id/api/v1/ports?genre=shooter&sort=populer'
```

---

## API admin

Panel admin dilayani di **`https://vps.gameport.web.id`**. Semua endpoint di
bawah `ADMIN_PATH` (path rahasia di `.env`). Butuh sesi cookie.

| Endpoint | Keterangan |
|---|---|
| `POST {ADMIN_PATH}/login` | `{"password":"..."}` → set cookie |
| `POST {ADMIN_PATH}/logout` | hapus sesi |
| `GET {ADMIN_PATH}/api/stat` | statistik + grafik + populer + pencarian |
| `GET {ADMIN_PATH}/api/list` | semua port, termasuk draf |
| `GET {ADMIN_PATH}/api/port/{id}` | detail untuk form edit |
| `POST {ADMIN_PATH}/api/port` | buat port baru |
| `PUT {ADMIN_PATH}/api/port/{id}` | ubah port |
| `DELETE {ADMIN_PATH}/api/port/{id}` | hapus port |
| `POST {ADMIN_PATH}/api/upload` | multipart `file` → `{"url":"/uploads/..."}` |

Payload port:

```json
{
  "game": "Half-Life (Xash3D FWGS)",
  "version": "1.3.0",
  "file_size": "790 MB",
  "genre_slug": "shooter",
  "description": "Engine GoldSrc kompatibel…",
  "author": "FWGS",
  "category_slug": "source-port",
  "status_slug": "stable",
  "image_url": "/uploads/xxx.png",
  "download_url": "https://example.com/hl.apk",
  "published": true,
  "needs_own_data": true,
  "platforms": ["android", "windows"]
}
```

Wajib: `game`, `category_slug`, `status_slug`.

---

## Database

**8 tabel + 2 view.**

| Tabel | Isi |
|---|---|
| `ports` | konten utama |
| `categories` | kategori port (source port, rekompilasi, …) |
| `statuses` | kematangan (stable, beta, alpha, experimental) |
| `platforms` | platform target |
| `genres` | genre game |
| `port_platforms` | relasi many-to-many port ↔ platform |
| `events` | catatan view & download |
| `admin_sessions` | sesi admin aktif |

View: `v_port_detail` (meratakan join jadi array platform),
`v_statistik` (agregat untuk beranda).

`db/seed.sql` **idempoten** — aman dijalankan berulang. Data referensi
(18 genre, 9 platform, 9 kategori, 4 status) di-`ON CONFLICT DO NOTHING`,
jadi isi katalog tidak terhapus.

---

## Keamanan

- Panel admin di path rahasia, tidak ditautkan dari halaman publik
- Endpoint admin membalas **404** (bukan 401) untuk pengunjung tanpa sesi —
  tidak mengumumkan keberadaan panel
- Password admin di-hash SHA-256, dibandingkan konstan-waktu
- Jeda 600 ms pada login gagal (memperlambat brute force)
- Upload diverifikasi magic bytes, bukan hanya ekstensi nama file
- Cookie `HttpOnly`, `SameSite=Lax`, `Secure` otomatis saat HTTPS
  (dibaca dari `X-Forwarded-Proto`)
- Halaman admin `no-store` + `noindex, nofollow`
- CORS hanya untuk `/api/` publik

**Kredensial tidak pernah masuk repo.** `.env` ada di `.gitignore`;
yang di-commit hanya `.env.example`.

---

## Pengembangan

```bash
go vet ./...
go build ./...
go run ./cmd/server
```

Butuh Go 1.23+. Semua konfigurasi lewat environment — lihat `.env.example`.

Build untuk produksi:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -trimpath -ldflags="-s -w" -o gameport ./cmd/server
```

---

## Catatan operasional

Hal-hal yang sudah pernah bikin masalah, dicatat supaya tidak terulang:

- **`.env` mengandung `&` atau `?`** → wajib di-quote waktu di-`source`
  shell. Contoh: `DATABASE_URL='postgres://...?host=/path&port=5432'`.
  Tanpa quote, `&` memecah perintah dan nilai terpotong.
- **`systemctl reload gameport` tidak membaca ulang `.env`.** Pakai
  `restart`. `daemon-reload` hanya perlu kalau file `.service` berubah.
- **Binary tidak bisa ditimpa saat berjalan** (`Text file busy`).
  Hentikan dulu, baru `cp`.
- **Bind ke `127.0.0.1`** membuat app tidak bisa dijangkau dari host.
  Untuk container, `0.0.0.0` wajib.
- **Saat mencari proses**, `cmdline` bisa berisi `./gameport` saja —
  cocokkan lewat `/proc/PID/exe` supaya tidak salah sasaran.
- **`ufw reload` cukup** untuk menerapkan rule; `systemctl restart ufw`
  jarang perlu dan lebih berisiko memutus SSH.
- **Port 80 dan 443 harus dibuka di dua tempat** kalau di VPS cloud:
  `ufw` di server **dan** Security Group di panel provider.

---

## Lisensi

Kode: MIT. Data port milik masing-masing pembuat aslinya.
