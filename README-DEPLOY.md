# GamePort — Panduan Deploy (cara lama: systemd di host)

> **Catatan:** dokumen ini menjelaskan cara lama — binary + systemd di host.
> Deployment produksi sekarang memakai frontend Vercel + backend container,
> artifact GitHub Release, dan auto-deploy tanpa git clone di server.
> Lihat **[deploy/README.md](deploy/README.md)** untuk alur yang dipakai
> sekarang. Dokumen ini tetap disimpan untuk referensi.

Panduan lengkap: dari VPS kosong sampai situs jalan dengan HTTPS.

```
game-port/
├── gameport          binary Go (statis, tanpa dependensi)
├── seed.sql          skema database (jalankan sekali)
├── .env.example      contoh konfigurasi
├── Caddyfile         konfigurasi reverse proxy + HTTPS otomatis
├── gameport.service  unit systemd
├── static/
│   ├── index.html    halaman publik
│   ├── app.js
│   ├── style.css
│   ├── admin.html    panel admin
│   ├── admin.js
│   └── admin.css
└── uploads/          gambar lama (fitur unggah sudah dihapus)
```

---

## 1. Siapkan PostgreSQL

```bash
sudo apt update
sudo apt install -y postgresql
sudo systemctl enable --now postgresql
```

Buat user dan database:

```bash
sudo -u postgres psql <<'SQL'
CREATE ROLE gameport WITH LOGIN PASSWORD 'GANTI_PASSWORD_KUAT';
CREATE DATABASE gamedb OWNER gameport;
SQL
```

Terapkan skema (sekali saja):

```bash
psql "postgres://gameport:GANTI_PASSWORD_KUAT@127.0.0.1/gamedb" -f seed.sql
```

Verifikasi:

```bash
psql "postgres://gameport:GANTI_PASSWORD_KUAT@127.0.0.1/gamedb" -c "\dt"
```

Harus muncul 8 tabel: `ports`, `categories`, `statuses`, `platforms`,
`genres`, `port_platforms`, `events`, `admin_sessions`.

---

## 2. Pasang aplikasi

```bash
# user khusus, tanpa shell login (lebih aman)
sudo useradd --system --home /srv/gameport --shell /usr/sbin/nologin gameport

# salin folder
sudo mkdir -p /srv/gameport
sudo cp -r gameport static uploads seed.sql .env.example Caddyfile gameport.service /srv/gameport/
sudo chown -R gameport:gameport /srv/gameport
sudo chmod +x /srv/gameport/gameport
```

Buat konfigurasi:

```bash
cd /srv/gameport
sudo -u gameport cp .env.example .env
sudo -u gameport nano .env
```

Isi yang **wajib** diganti:

```bash
GAMEDB_PASSWORD=ganti_password_kuat
ADMIN_PASSWORD=ganti_password_admin_kuat
ADMIN_PATH=/panel-a8f3k2m9      # ganti dengan string acak milikmu
```

> **Penting:** `ADMIN_PATH` adalah alamat panel admin. Jangan pakai
> `/admin` — itu yang pertama dicoba orang. Pakai sesuatu seperti
> `/panel-a8f3k2m9` yang hanya kamu tahu.

Uji jalan dulu sebelum masuk systemd:

```bash
cd /srv/gameport
sudo -u gameport env $(cat .env | grep -v '^#' | xargs) ./gameport
```

Buka di browser (dari server): `curl http://127.0.0.1:8181/api/health`

Hentikan dengan `Ctrl+C`.

---

## 3. Jalankan sebagai service

```bash
sudo cp /srv/gameport/gameport.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now gameport
```

Cek status:

```bash
sudo systemctl status gameport
sudo journalctl -u gameport -n 30 --no-pager
```

Kalau gambar lama tidak tampil, pastikan folder `uploads` bisa dibaca:

```bash
sudo chown -R gameport:gameport /srv/gameport/uploads
```

---

## 4. Pasang Caddy (HTTPS otomatis)

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
  | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
  | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install -y caddy
```

Edit Caddyfile, ganti domain:

```bash
sudo nano /srv/gameport/Caddyfile
```

```caddyfile
gameport.domainmu.com {
    encode zstd gzip

    request_body {
        max_size 8MB
    }

    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
        -Server
    }

    @admin path /panel-a8f3k2m9*
    header @admin X-Robots-Tag "noindex, nofollow, noarchive"

    reverse_proxy 127.0.0.1:8181 {
        header_up X-Forwarded-Proto {scheme}
        header_up X-Real-IP {remote_host}
    }

    @statis path /static/* /uploads/*
    header @statis Cache-Control "public, max-age=604800"
}
```

Pasang dan jalankan:

```bash
sudo cp /srv/gameport/Caddyfile /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

**Prasyarat HTTPS:** domain harus sudah mengarah ke IP server (record `A`),
dan port **80** serta **443** terbuka di firewall. Caddy akan membuat
sertifikat Let's Encrypt otomatis dalam hitungan detik.

Kalau belum punya domain, pakai ini untuk uji:

```caddyfile
:80 {
    reverse_proxy 127.0.0.1:8181
}
```

---

## 5. Pakai panel admin

Buka `https://domainmu.com/panel-x7k2` (sesuai `ADMIN_PATH` di `.env`).

Masukkan `ADMIN_PASSWORD`. Setelah masuk:

- **Statistik** — total port, view, unduh, grafik 14 hari, port paling
  dilihat, pencarian teratas
- **Daftar Port** — tabel dengan edit dan hapus
- **Tambah Port** — form lengkap: nama, versi, ukuran, genre, kategori,
  status, author, deskripsi, platform, gambar, link unduhan

Field yang wajib: **nama game**, **kategori**, **status**. Sisanya opsional.

**Gambar:** tempel URL gambar langsung (mis. `https://contoh.com/sampul.jpg`).
Tidak ada unggahan berkas; gambar lama di `uploads/` tetap disajikan lewat
`/uploads/nama-file`.

**Terbitkan:** jika tidak dicentang, port berstatus **draf** — tersimpan
di admin tapi tidak muncul di halaman publik.

---

## 6. Firewall

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

**Jangan** buka port 8181 ke publik — Caddy yang meneruskan ke sana
lewat `127.0.0.1`.

---

## 7. Backup

Database:

```bash
pg_dump "postgres://gameport:PASS@127.0.0.1/gamedb" | gzip > gamedb-$(date +%F).sql.gz
```

Gambar:

```bash
tar czf uploads-$(date +%F).tar.gz -C /srv/gameport uploads
```

Jadwalkan harian lewat cron:

```bash
sudo crontab -e
```

```cron
0 3 * * * pg_dump "postgres://gameport:PASS@127.0.0.1/gamedb" | gzip > /var/backups/gamedb-$(date +\%F).sql.gz
```

---

## 8. Update aplikasi

```bash
# ganti binary
sudo systemctl stop gameport
sudo cp gameport.new /srv/gameport/gameport
sudo chown gameport:gameport /srv/gameport/gameport
sudo chmod +x /srv/gameport/gameport
sudo systemctl start gameport
```

Kalau ada perubahan skema, jalankan `seed.sql` lagi (idempoten, aman).

---

## Troubleshooting

**Service gagal start**

```bash
sudo journalctl -u gameport -n 50 --no-pager
```

Penyebab umum:
- `ADMIN_PASSWORD` belum diisi → server menolak start (memang disengaja)
- Password DB salah
- Folder `uploads` tidak bisa dibaca (hanya untuk gambar lama)

**Admin tidak bisa login**

- Pastikan `ADMIN_PATH` di `.env` sama dengan URL yang dibuka
- Cookie butuh HTTPS. Kalau lewat IP langsung (HTTP), cookie `Secure`
  tidak terpasang — pakai domain dengan HTTPS

**Gambar tidak muncul**

- Gambar baru diisi URL eksternal — cek URL-nya bisa diakses publik
- Untuk gambar lama: cek `ls -la /srv/gameport/uploads`
- Pastikan folder bisa dibaca user `gameport`
- Coba akses langsung `https://domainmu.com/uploads/nama-file.png`

**Caddy gagal dapat sertifikat**

- DNS belum propagasi: `dig domainmu.com`
- Port 80/443 tertutup
- Cek log: `sudo journalctl -u caddy -n 50`

**"out of host capacity" saat beli VPS** — bukan masalah aplikasi, itu
kapasitas provider.

---

## Catatan keamanan

1. **Ganti `ADMIN_PATH`** dari nilai contoh. Ini satu-satunya lapisan
   pertama — panel tidak ditautkan dari halaman publik dan mengembalikan
   404 untuk pengunjung yang tidak login.
2. **Ganti `ADMIN_PASSWORD`** dengan yang kuat. Ada jeda 600 ms setiap
   login gagal untuk memperlambat brute force.
3. Sesi admin berlaku **12 jam**, disimpan di database, dan dihapus saat
   logout.
4. Gambar sampul diisi URL eksternal; tidak ada endpoint unggah berkas.
5. Semua data port (`published = true`) bisa diakses publik; yang draf
   hanya lewat admin.