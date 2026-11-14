# Setup Domain — gameport.web.id

> **Arsitektur saat ini:** apex `gameport.web.id` dan `www` dilayani Vercel.
> Hanya `api.gameport.web.id` dan `vps.gameport.web.id` yang mengarah ke VPS
> dan punya site block di `Caddyfile.domain`. Tabel DNS di bawah adalah setup
> lama (frontend monolitik di VPS) — jangan dipakai apa adanya untuk apex.
> Lihat **[deploy/README.md](deploy/README.md)** untuk alur deploy sekarang.

Panduan dari domain kosong sampai HTTPS aktif.

```
Domain : gameport.web.id
IP VPS : 149.41.192.124
```

---

## 0. Cek status domain dulu (JANGAN dilewati)

Sebelum apa pun, pastikan domainnya sudah ada di zona `.id`. Dari mana saja:

```bash
dig +short A gameport.web.id
```

- **Keluar IP** → domain aktif, lanjut ke langkah 2.
- **Kosong** → belum ada A record, lihat langkah 1.
- **NXDOMAIN** → domain belum terdaftar di registry. Hubungi registrar.

Cek juga nameserver-nya:

```bash
dig +short NS gameport.web.id
```

Kalau kosong atau `NXDOMAIN`, domain belum didelegasikan. Biasanya karena
registrasi belum selesai/terverifikasi di registrar, atau nameserver belum
diisi.

---

## 1. Isi DNS di registrar

Masuk ke panel registrar tempat kamu beli `gameport.web.id`, buka
**DNS Management / Manage DNS**, lalu tambahkan:

| Type | Name (Host) | Value | TTL |
|---|---|---|---|
| `A` | `@` | `149.41.192.124` | 300 |
| `CNAME` | `www` | `gameport.web.id.` | 300 |

Catatan:

- `@` artinya domain itu sendiri (`gameport.web.id`).
- Kalau registrar tidak menerima `@`, coba kosongkan, atau isi `gameport.web.id`.
- Titik di akhir `gameport.web.id.` itu penting di beberapa panel — kalau
  ditolak, tulis tanpa titik.
- **Jangan** tambahkan `AAAA` (IPv6) kecuali VPS-mu memang punya IPv6.
  AAAA yang salah membuat sebagian pengunjung gagal konek.

**Kalau pakai Cloudflare:**

- Saat pertama kali, set ikon awan jadi **abu-abu (DNS only)**. Let's Encrypt
  butuh melihat server asli untuk verifikasi HTTP-01.
- Setelah sertifikat terbit dan HTTPS aktif, boleh dinyalakan proxy-nya dan
  diatur ke SSL mode **Full (strict)**.

---

## 2. Buka port 80 dan 443 — di DUA tempat

Ini penyebab paling sering gagal, dan pesan errornya menyesatkan.

**a. Firewall di server (UFW):**

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw reload
sudo ufw status verbose
```

**b. Security Group di console Tencent Cloud:**

CVM → Instance → **Security Groups** → **Edit Rules** → **Add Rule**, dua kali:

| Type | Protocol | Port | Source | Policy |
|---|---|---|---|---|
| Custom | TCP | `80` | `0.0.0.0/0` | Allow |
| Custom | TCP | `443` | `0.0.0.0/0` | Allow |

Tanpa langkah ini, UFW boleh terbuka tapi trafik tetap diblokir di lapisan
cloud, dan Caddy akan gagal verifikasi terus-menerus.

Cek dari luar (dari laptopmu, bukan server):

```bash
nc -zv 149.41.192.124 80
nc -zv 149.41.192.124 443
```

---

## 3. Pasang Caddyfile domain

```bash
sudo cp /opt/alpha/hermes/workspace/deployed/game-port/Caddyfile.domain \
        /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Perbedaan penting dari versi IP:

| | Akses lewat IP | Akses lewat domain |
|---|---|---|
| Site address | `:80` | `gameport.web.id` |
| HTTPS | tidak ada | **otomatis** |
| Port yang dipakai | 80 | 80 (verifikasi) + 443 |

---

## 4. Tunggu sertifikat terbit

```bash
sudo journalctl -u caddy -f
```

Yang ditunggu, muncul dalam 5–60 detik:

```
certificate obtained successfully  identifier=gameport.web.id
```

Kalau gagal, log akan menyebut alasannya:

| Pesan | Artinya |
|---|---|
| `no such host` | DNS belum mengarah / belum propagasi |
| `connection refused` | Port 80 diblokir Security Group |
| `timeout` | Port 80 diblokir, atau DNS menunjuk IP lain |
| `unauthorized` | Ada proxy/CDN di depan (Cloudflare) — set DNS-only |
| `too many certificates` | Kena rate limit; tunggu 1 minggu |

---

## 5. Verifikasi

**Dari server:**

```bash
curl -sI https://gameport.web.id/ | head -3
```

**Cek redirect HTTP → HTTPS:**

```bash
curl -sI http://gameport.web.id/ | head -3
# harus 308, Location: https://gameport.web.id/
```

**Cek sertifikatnya benar:**

```bash
echo | openssl s_client -connect gameport.web.id:443 -servername gameport.web.id 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates
```

Harus `issuer=... Let's Encrypt ...` dan tanggal `notAfter` ±3 bulan ke depan.

**Dari luar server** (laptop/Hp, pakai data seluler):

```
https://gameport.web.id
https://gameport.web.id/admin
```

---

## 6. Setelah HTTPS aktif

**Panel admin sekarang aman** — cookie sesi otomatis ditandai `Secure`
karena GamePort membaca header `X-Forwarded-Proto: https` dari Caddy.

**HSTS sudah aktif** (dikirim di header). Artinya browser akan **menolak**
akses `http://gameport.web.id` selama setahun. Kalau suatu saat butuh
mematikan HTTPS, hapus baris `Strict-Transport-Security` dan bersihkan HSTS
di browser (`chrome://net-internals/#hsts`).

**Port 8181 tidak perlu dibuka** dan jangan dibuka. Caddy meneruskan ke
container lewat jaringan internal Docker (`172.18.0.3`).

---

## Troubleshooting

**`dig` kosong tapi registrar bilang aktif**

Propagasi butuh waktu. TTL 300 detik biasanya cukup, tapi NS baru bisa
sampai 24 jam. Cek dari resolver lain:

```bash
dig +short A gameport.web.id @8.8.8.8
dig +short A gameport.web.id @1.1.1.1
```

**Caddy minta sertifikat terus tanpa henti**

Cek apakah ada proses lain di port 80:

```bash
sudo ss -tlnp | grep -E ':80|:443'
```

Harus hanya `caddy`. Kalau ada nginx/apache, matikan dulu.

**Sertifikat terbit tapi browser bilang "Not Secure"**

Biasanya karena masih mengakses IP, bukan domain. Atau ada Cloudflare dengan
mode **Flexible** — ubah ke **Full (strict)**.

**Mau pindah dari IP ke domain**

Ganti `/etc/caddy/Caddyfile` dengan `Caddyfile.domain`, reload. Tidak perlu
ubah apa pun di GamePort, karena `APP_HOST` tetap `0.0.0.0:8181` di dalam
container dan Caddy yang menangani domain.
