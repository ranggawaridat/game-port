# Frontend GamePort

Situs publik + panel admin. Murni statis — semua data lewat API JSON.

## Konfigurasi origin API

Di `app.js`, bagian atas:

```js
const SAMA_ORIGIN = location.hostname !== 'gameport.web.id';
const API_ORIGIN  = SAMA_ORIGIN ? '' : 'https://api.gameport.web.id';
const API         = API_ORIGIN + '/api/v1';
```

- Saat halaman dilayani **backend sendiri** (dev atau masa monolitik),
  hostname bukan `gameport.web.id` → base kosong → semua URL relative.
- Saat halaman dilayani **Vercel** di `gameport.web.id` → origin API eksplisit,
  karena memanggil `api.gameport.web.id` berarti lintas origin.

Kalau nanti domain atau subdomain berubah, cukup ubah dua konstanta itu.

## Jalur yang sengaja tetap relative

`/port/{slug}` adalah **route frontend** (Vercel rewrite → `index.html`),
bukan API — lihat `vercel.json`.

`/admin/api/*` tidak ada di sini: `admin.js` memakai path relative karena
panel admin dilayani dari `vps.gameport.web.id` yang di-proxy ke backend —
satu origin, jadi cookie sesi tetap bekerja tanpa CORS.

## Deploy ke Vercel

1. Import repo ini ke Vercel, **Root Directory**: `frontend`
2. Framework Preset: **Other** (statis murni, tanpa build)
3. Pastikan DNS: record `A`/`CNAME` `gameport.web.id` menunjuk ke Vercel

`vercel.json` sudah berisi rewrite `/port/* → index.html` supaya halaman
detail bisa dibagikan sebagai URL langsung.

## Catatan

- Tidak ada build step, tidak ada bundler — sengaja.
- `vercel.json` hanya rewrite + header keamanan dasar.
- Script iklan safelink TIDAK di sini; ia di panel admin
  (`vps.gameport.web.id/admin` → tab Safelink), tersimpan di database.
