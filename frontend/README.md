# Frontend GamePort

Situs publik + panel admin. Murni statis — semua data lewat API JSON.

## Konfigurasi origin API

Di `app.js`, bagian atas:

```js
const API_ORIGIN = 'https://api.gameport.web.id';
const API        = `${API_ORIGIN}/api/v1`;
```

Situs publik selalu memanggil `api.gameport.web.id` (lintas origin dari
Vercel). Kalau domain/subdomain berubah, ubah `API_ORIGIN` dan pastikan origin
baru masuk `CORS_ORIGINS` di `.env` backend.

## Jalur yang sengaja tetap relative

`/port/{slug}`, `/genre/{slug}`, `/platform/{slug}`, dan `/kategori/{slug}`
adalah **route frontend** (Vercel rewrite → `index.html`), bukan API — lihat
`vercel.json`. Metadata tiap halaman (title, description, canonical, Open
Graph, JSON-LD) diisi `app.js` setelah data API tiba.

`/sitemap.xml` di-rewrite ke backend (`api.gameport.web.id/sitemap.xml`) agar
isinya selalu mengikuti database. `robots.txt` statis menunjuk ke sana.
Gambar pratinjau sosial default: `og-default.png` (1200×630).

`/admin/api/*` tidak ada di sini: `admin.js` memakai path relative karena
panel admin dilayani dari `vps.gameport.web.id` yang di-proxy ke backend —
satu origin, jadi cookie sesi tetap bekerja tanpa CORS.

## Deploy ke Vercel

1. Import repo ini ke Vercel, **Root Directory**: `frontend`
2. Framework Preset: **Other** (statis murni, tanpa build)
3. Pastikan DNS: record `A`/`CNAME` `gameport.web.id` menunjuk ke Vercel

`vercel.json` sudah berisi rewrite `/port/*`, `/genre/*`, `/platform/*`, dan
`/kategori/*` → `index.html` supaya halaman detail dan landing page bisa
dibagikan sebagai URL langsung, plus rewrite `/sitemap.xml` ke backend.

## Catatan

- Tidak ada build step, tidak ada bundler — sengaja.
- `vercel.json` hanya rewrite + header keamanan dasar.
- Script iklan safelink TIDAK di sini; ia di panel admin
  (`vps.gameport.web.id` + `ADMIN_PATH` → tab Safelink), tersimpan di database.
