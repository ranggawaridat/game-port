# SEO GamePort

Catatan ini merangkum optimasi yang sudah terpasang dan langkah manual yang
masih perlu dilakukan di luar repo (Search Console, sosial, iklan).

## Yang sudah ada di kode

| Bagian | Lokasi | Fungsi |
| --- | --- | --- |
| `robots.txt` | `frontend/robots.txt` | Mengizinkan crawl, menunjuk sitemap, memberi akses ke bot sosial. |
| `sitemap.xml` dinamis | `backend/internal/api/seo.go` + rewrite `frontend/vercel.json` | Berisi beranda, semua port terbit, genre, platform, kategori. Selalu ikut database. |
| Meta per halaman | `frontend/app.js` (`applySeo`) | `title`, `description`, `canonical`, Open Graph, Twitter Card, `robots` per halaman port dan landing page. |
| JSON-LD | `frontend/app.js`, `frontend/index.html` | `WebSite` + `SearchAction`, `Organization`, `SoftwareApplication` + `BreadcrumbList` untuk port, `CollectionPage` + `ItemList` untuk landing page. |
| Landing page long-tail | `/genre/{slug}`, `/platform/{slug}`, `/kategori/{slug}` | Halaman terindeks dengan teks unik Indonesia, daftar port, dan tautan internal. |
| Tautan internal | kartu, breadcrumb, footer | Setiap kategori/genre/platform di kartu dapat diklik; footer memuat seluruh genre, platform, dan kategori. |
| Kartu sosial default | `frontend/og-default.png` (1200×630) | Pratinjau saat beranda dibagikan. |
| Share button | halaman detail | WhatsApp, Telegram, X, Facebook, salin tautan. |
| Filter di URL | `app.js` (`tulisStateKeURL`) | `/?q=...&genre=...` bisa dibagikan; mendukung `SearchAction` di JSON-LD. |
| Performa | `index.html` | `preconnect` ke font & API, `display=swap`, `width`/`height` gambar, `loading=lazy`. |

Catatan: konten tetap dirender di browser (tanpa SSR). Google dan Bing modern
menjalankan JavaScript sehingga membaca meta di atas. Bot sosial hanya membaca
HTML statis, jadi tautan yang dibagikan memakai kartu default, bukan judul per
game. Kalau nanti ingin kartu sosial per game, perlu SSR di backend atau
fungsi serverless di Vercel.

## Langkah manual setelah deploy

1. **Deploy backend** (GitHub Release → autodeploy) agar `/sitemap.xml` hidup.
2. **Deploy frontend** ke Vercel agar rewrite dan `robots.txt` aktif.
3. Cek:
   - `https://gameport.web.id/sitemap.xml` → XML berisi URL port.
   - `https://gameport.web.id/robots.txt` → menunjuk sitemap.
   - `https://api.gameport.web.id/sitemap.xml` → sumber langsung.
4. **Google Search Console** — tambahkan properti domain `gameport.web.id`
   (verifikasi DNS TXT, tidak perlu ubah kode), lalu *Sitemaps* → kirim
   `sitemap.xml`.
5. **Bing Webmaster Tools** — impor dari Search Console (opsional tapi cepat).
6. **Uji kartu sosial**: Facebook Sharing Debugger, Telegram `@WebpageBot`,
   dan WhatsApp (kirim ke diri sendiri). Jika gambar tidak muncul, paksa
   *scrape ulang* di debugger.
7. **Analytics** minimal: Google Analytics 4 atau Vercel Web Analytics. Tanpa
   data, keputusan konten hanya tebakan.

## Prioritas pertumbuhan organik

1. **Isi katalog, bukan jumlah halaman kosong.** Halaman port tanpa deskripsi
   dan tanpa gambar hampir tidak akan ranking. Target: setiap port punya
   deskripsi 2–4 kalimat, gambar sampul, versi, ukuran, dan catatan instalasi.
2. **Tangkap kata kunci long-tail.** Nama game + "android port"/"apk" adalah
   pencarian paling realistis. Judul halaman sudah memakai pola
   `{Game} — Port Android`. Pastikan nama game di admin memakai ejaan yang
   dicari orang (mis. "GTA San Andreas", bukan singkatan internal).
3. **Perbanyak halaman gabungan.** Setiap kombinasi genre/platform yang punya
   ≥3 port layak jadi halaman. Setelah katalog membesar, pertimbangkan halaman
   seperti `/genre/rpg/platform/android` (belum dibuat).
4. **Konten pendukung.** Artikel panduan ("cara memasang port Android",
   "game PC yang sudah di-port ke Android") menangkap trafik informasi dan
   menaut ke halaman port. Ini yang biasanya mendorong lonjakan awal.
5. **Segarkan halaman lama.** Sitemap memakai `lastmod` dari `updated_at`;
   perbarui deskripsi/versi port secara berkala agar crawler memindai ulang.
6. **Internal link.** Tautkan antar port sejenis di deskripsi (mis. "mirip
   Quake"), dan pastikan setiap port punya minimal satu tautan masuk dari
   landing page.

## Non-organik (berbayar & sosial)

- **Sosial**: setiap halaman punya tombol share. Pola yang efektif: potong
  video 15 detik gameplay di HP, tautkan ke halaman port. Kartu default sudah
  tampil rapi.
- **Komunitas**: forum/Reddit/Discord port Android dan grup Telegram. Hindari
  spam tautan; bagikan hanya saat ada port baru yang benar-benar berfungsi.
- **Iklan berbayar**: mulai dari Search Ads dengan kata kunci merek game +
  "android", bukan kata kunci umum. Halaman tujuan sudah punya tombol unduh
  yang jelas.
- **Safelink** (fitur iklan yang sudah ada) adalah monetisasi, bukan akuisisi.
  Jangan aktifkan pada trafik organik baru sebelum halaman port lengkap —
  gerbang iklan menurunkan konversi dan sinyal kualitas.

## Yang perlu dihindari

- Mengindeks halaman filter kosong. Landing page dengan 0 port tidak masuk
  sitemap dan tautannya tidak dirender.
- Menyalin deskripsi dari situs lain; Google akan menganggapnya duplikat.
- Menumpuk kata kunci di footer. Tautan footer dibatasi pada genre, platform,
  dan kategori yang benar-benar ada.
- Membuat halaman `/port/...` versi API (`api.gameport.web.id`) ikut terindeks;
  kanonik selalu menunjuk ke `gameport.web.id`.
