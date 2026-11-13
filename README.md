# GamePort

Katalog port native Android dari game PC & konsol — source port, dekompilasi,
dan rekompilasi statis yang menjalankan kode game langsung di perangkat,
bukan emulasi.

**Backend Go · PostgreSQL · frontend vanilla HTML/CSS/JS · panel admin bawaan**

Konten diisi **manual** lewat panel admin. Tidak ada scraping.

![GamePort](docs/screenshot.png)

---

## Fitur

**Publik**
- Katalog dengan gambar sampul, versi, ukuran berkas, genre, dan platform
- Pencarian teks, filter kategori/genre/platform, 5 mode pengurutan
- Hitungan tampilan dan unduhan per game
- Responsif, tema gelap, tanpa framework

**Admin** (path rahasia, tidak ditautkan dari publik)
- Statistik: total view, unduh, grafik 14 hari, game paling dilihat, pencarian teratas
- CRUD lengkap: tambah, edit, hapus port
- Unggah gambar sampul (maks 5 MB, diverifikasi magic bytes)
- Status draf — konten bisa disimpan tanpa dipublikasikan
- Sesi cookie 12 jam, disimpan di database

---

## Jalankan cepat

```bash
cp .env.example .env     # isi GAMEDB_PASSWORD + ADMIN_PASSWORD
docker compose up -d --build
```

Buka <http://127.0.0.1:8181>. Panel admin di `http://127.0.0.1:8181/panel-x7k2`
(sesuai `ADMIN_PATH`).

**Panduan deploy lengkap** (VPS, PostgreSQL, systemd, Caddy + HTTPS):
baca **[README-DEPLOY.md](README-DEPLOY.md)**.

---

## Dua cara deploy

| | Cara A: systemd di host | Cara B: di dalam container |
|---|---|---|
| Aplikasi jalan di | host (`/srv/gameport`) | container Hermes |
| Database | PostgreSQL host | PostgreSQL container (unix socket) |
| Caddy meneruskan ke | `127.0.0.1:8181` | `172.18.0.3:8181` |
| Cocok untuk | VPS khusus GamePort | Hermes + GamePort satu VPS |

**Cara B** tidak butuh user sistem, tidak butuh `systemctl`, dan database-nya
hanya lewat UNIX socket sehingga tidak ada port DB yang terbuka.

```bash
# di host
docker exec -u hermes alpha-hermes /opt/data/gameport/manager.sh start

# Caddyfile untuk cara B
sudo cp Caddyfile.ip /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

`deploy/manager.sh` mengurus PostgreSQL + GamePort sekaligus
(`start` / `stop` / `restart` / `status` / `logs`).

> **Penting saat pakai IP mentah:** tulis site address Caddy sebagai `:80`,
> **bukan** `IP_PUBLIK { ... }`. Kalau IP ditulis sebagai site address, Caddy
> memaksa HTTPS dan membalas **308 redirect** ke port 443 — yang gagal bila
> 443 tertutup atau sertifikat tidak ada. Lihat catatan di `Caddyfile.ip`.

---

## Struktur

```
cmd/server/main.go          entrypoint, graceful shutdown
internal/
  config/config.go          konfigurasi env, escape kredensial DSN
  db/
    db.go                   pool pgx + health
    models.go               tipe JSON
    queries.go              SQL, filter, paginasi, mutasi, analitik
  api/
    server.go               router publik, static, CORS
    admin.go                CRUD admin, upload, statistik
    auth.go                 sesi cookie, verifikasi password
static/
  index.html app.js style.css        frontend publik
  admin.html admin.js admin.css      panel admin
db/seed.sql                 skema + referensi (idempoten)
Caddyfile                   reverse proxy + HTTPS
gameport.service            unit systemd
```

---

## API publik

| Endpoint | Keterangan |
|---|---|
| `GET /api/health` | status koneksi DB + pool |
| `GET /api/statistik` | agregat katalog |
| `GET /api/kategori` | kategori + jumlah |
| `GET /api/status` | status kematangan + jumlah |
| `GET /api/platform` | platform + jumlah |
| `GET /api/genre` | genre + jumlah |
| `GET /api/ports` | daftar port (filter, sort, paginasi) |
| `GET /api/ports/{slug}` | detail + 6 terkait |
| `GET /api/ports/{slug}/download` | catat unduhan, lalu redirect |

Parameter `/api/ports`: `q`, `kategori`, `genre`, `platform`, `own=0|1`,
`sort` (`kategori` `nama` `nama-desc` `status` `populer` `terbaru` `download`),
`limit` (1–200), `offset`.

```bash
curl 'http://127.0.0.1:8181/api/ports?q=half-life'
curl 'http://127.0.0.1:8181/api/ports?genre=shooter&sort=populer'
```

---

## API admin

Semua di bawah `ADMIN_PATH` (contoh `/panel-x7k2`). Butuh sesi cookie.

| Endpoint | Keterangan |
|---|---|
| `POST {ADMIN_PATH}/login` | `{"password":"..."}` → set cookie |
| `POST {ADMIN_PATH}/logout` | hapus sesi |
| `GET {ADMIN_PATH}/api/stat` | statistik + grafik + populer + pencarian |
| `GET {ADMIN_PATH}/api/list` | daftar semua port (termasuk draf) |
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

Tabel: `ports` (konten), referensi (`categories`, `statuses`, `platforms`,
`genres`), relasi (`port_platforms`), analitik (`events`, `admin_sessions`).

Dua view: `v_port_detail` (meratakan join jadi array) dan `v_statistik`
(agregat). `db/seed.sql` idempoten — aman dijalankan berulang.

---

## Keamanan

- Panel admin di path acak, tidak ditautkan dari halaman publik
- Endpoint admin mengembalikan **404** (bukan 401) untuk pengunjung tanpa sesi
- Password admin di-hash SHA-256 dan dibandingkan konstan-waktu
- Jeda 600 ms pada login gagal (memperlambat brute force)
- Upload diverifikasi magic bytes, bukan hanya ekstensi
- Cookie `HttpOnly`, `SameSite=Lax`, `Secure` otomatis saat HTTPS
- CORS hanya untuk `/api/` publik; halaman admin `no-store` + `noindex`

---

## Pengembangan

```bash
go vet ./...
go build ./...
go run ./cmd/server
```

Butuh Go 1.23+. Konfigurasi lewat environment, lihat `.env.example`.

---

## Lisensi

Kode: MIT. Data port milik masing-masing pembuat aslinya.
