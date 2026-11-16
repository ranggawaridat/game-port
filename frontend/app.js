/* GamePort — frontend publik. Semua data dari API. */
'use strict';

/* ---------- Konfigurasi origin ----------
 * Satu domain: frontend, API, dan admin semuanya di gameport.web.id,
 * disajikan backend Go di belakang Caddy. Jadi API dipanggil relative —
 * tidak ada CORS, dan cookie admin tetap satu origin.
 *
 * Bila suatu saat frontend dipindah ke host terpisah (Vercel), isi
 * API_ORIGIN dengan origin backend-nya, mis. 'https://api.gameport.web.id'.
 */
const API_ORIGIN = '';
const API = `${API_ORIGIN}/api/v1`;
const SITE = 'https://gameport.web.id';
const DEFAULT_TITLE = 'GamePort — Port Native Android dari Game PC & Konsol';
const DEFAULT_DESC = 'Katalog port native Android dari game PC dan konsol: source port, dekompilasi, dan rekompilasi statis. Unduh APK dan mainkan tanpa emulator.';
const DEFAULT_IMAGE = SITE + '/og-default.png';

const state = { q:'', kategori:'', status:'', platform:'', genre:'', sort:'terbaru', limit:24, offset:0 };
const $ = (s) => document.querySelector(s);
// Label versi: tag GitHub kadang sudah berawalan "v" (v0.8.8),
// jadi jangan menambah "v" kedua kali.
const verLabel = (v) => { const s = String(v ?? '').trim(); return s ? 'v' + s.replace(/^v/i, '') : ''; };
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

const asetAPI = (u) => (u && u.startsWith('/') ? API_ORIGIN + u : u);

const STATUS_STYLE = {
  stable:       { bg:'rgba(34,197,94,.9)',  fg:'#04140a' },
  beta:         { bg:'rgba(245,158,11,.9)', fg:'#1a1003' },
  experimental: { bg:'rgba(234,88,12,.9)',  fg:'#180a02' },
  alpha:        { bg:'rgba(239,68,68,.9)',  fg:'#170404' }
};
const PLATFORM_LABEL = { android:'Android', windows:'Windows', linux:'Linux', macos:'macOS',
  ios:'iOS','steam-deck':'Steam Deck', web:'Web', quest:'Quest', switch:'Switch' };

/* ═══════════════════════════════════════════════════════════════════════
   SEO — metadata per halaman
   Halaman dirender di browser, jadi tag <head> diperbarui saat data tiba.
   Google membaca hasil render ini; bot sosial memakai tag statis di
   index.html. Judul, deskripsi, canonical, OG, dan JSON-LD dijaga seragam.
   ═══════════════════════════════════════════════════════════════════════ */

function upsertMeta(attr, key, content) {
  let tag = document.head.querySelector(`meta[${attr}="${key}"]`);
  if (!tag) {
    tag = document.createElement('meta');
    tag.setAttribute(attr, key);
    document.head.appendChild(tag);
  }
  tag.setAttribute('content', content);
}

function upsertLink(rel, href) {
  let tag = document.head.querySelector(`link[rel="${rel}"]`);
  if (!tag) {
    tag = document.createElement('link');
    tag.setAttribute('rel', rel);
    document.head.appendChild(tag);
  }
  tag.setAttribute('href', href);
}

function setJsonLd(data) {
  const lama = document.getElementById('ld-page');
  if (lama) lama.remove();
  if (!data) return;
  const s = document.createElement('script');
  s.type = 'application/ld+json';
  s.id = 'ld-page';
  // Escape '<' agar nama game/deskripsi tidak bisa menutup tag <script>.
  s.textContent = JSON.stringify(data).replace(/</g, '\\u003c');
  document.head.appendChild(s);
}

function applySeo({ title, description, path, image, type = 'website', robots, jsonLd }) {
  const url = SITE + (path || '/');
  const desc = description || DEFAULT_DESC;
  const img = image || DEFAULT_IMAGE;
  const judul = title || DEFAULT_TITLE;

  document.title = judul;
  upsertMeta('name', 'description', desc);
  upsertMeta('name', 'robots', robots || 'index, follow, max-image-preview:large, max-snippet:-1');
  upsertLink('canonical', url);

  upsertMeta('property', 'og:type', type);
  upsertMeta('property', 'og:title', judul);
  upsertMeta('property', 'og:description', desc);
  upsertMeta('property', 'og:url', url);
  upsertMeta('property', 'og:image', img);
  upsertMeta('property', 'og:image:alt', judul);

  upsertMeta('name', 'twitter:card', 'summary_large_image');
  upsertMeta('name', 'twitter:title', judul);
  upsertMeta('name', 'twitter:description', desc);
  upsertMeta('name', 'twitter:image', img);

  setJsonLd(jsonLd);
}

/* Potong teks ke batas meta description tanpa memotong kata. */
function ringkas(teks, maks = 157) {
  const s = String(teks || '').replace(/\s+/g, ' ').trim();
  if (s.length <= maks) return s;
  const potong = s.slice(0, maks);
  const i = potong.lastIndexOf(' ');
  return (i > 40 ? potong.slice(0, i) : potong).replace(/[.,;:!?\-–—]+$/, '') + '…';
}

/* ═══════════════════ teks unik untuk landing page ═══════════════════ */

const GENRE_INFO = {
  action: 'Genre action menuntut refleks cepat dan kontrol presisi. Port native action biasanya memakai ulang engine asli, sehingga animasi dan fisika tetap sama seperti versi PC — hanya kontrolnya yang dipetakan ulang untuk layar sentuh atau gamepad.',
  adventure: 'Petualangan berbasis cerita, eksplorasi, dan teka-teki. Port adventure dari game klasik sering hadir sebagai reimplementasi engine (scummvm dan sejenisnya), jadi dialog, inventori, dan save game berjalan apa adanya di Android.',
  rpg: 'RPG menawarkan sistem statistik, dialog bercabang, dan progresi panjang. Port native RPG umumnya hasil dekompilasi atau reimplementasi engine; dukungan simpan game, mod, dan gamepad jadi pertimbangan utama sebelum memasang.',
  strategy: 'Strategi real-time maupun turn-based butuh antarmuka padat dan peta luas. Di Android, port native genre ini paling nyaman dengan layar minimal 6 inci atau tablet, dan beberapa proyek menambahkan antarmuka khusus sentuh.',
  shooter: 'Shooter klasik seperti Doom, Quake, dan Duke Nukem punya source port matang di Android. Dukungan mouse, gamepad, dan server multiplayer tetap tersedia, menjadikannya pilihan populer untuk main di perangkat genggam.',
  racing: 'Game balap dengan fisika kendaraan dan lintasan besar. Port native biasanya menyertakan pengaturan grafis dan pemetaan kontrol roda kemudi sentuh; performa sangat bergantung pada GPU perangkat.',
  sports: 'Olahraga seperti sepak bola, basket, dan skate. Port native mempertahankan aturan dan AI asli, sehingga pengalaman bermain lebih konsisten dibanding emulasi konsol.',
  simulation: 'Simulasi kendaraan, kota, dan penerbangan. Genre ini paling sensitif terhadap performa, jadi cek status kematangan port dan kebutuhan RAM sebelum memasang.',
  puzzle: 'Puzzle logika, blok, dan teka-teki ringan. Port native puzzle umumnya berukuran kecil dan berjalan lancar bahkan di perangkat lama, cocok untuk sesi singkat.',
  platformer: 'Lompat, lari, dan eksplorasi level. Port platformer klasik hasil rekompilasi mendapat kontrol sentuh yang dipetakan ulang; gamepad sangat disarankan untuk level yang menuntut presisi.',
  fighting: 'Pertarungan satu lawan satu dengan frame data ketat. Port native fighting butuh latensi rendah; gunakan gamepad Bluetooth dan mode performa tinggi agar input tidak tertinggal.',
  horror: 'Survival horror dan horor atmosferik. Port native mempertahankan pencahayaan dan tata suara asli; beberapa judul membutuhkan data game asli karena engine-nya dirilis terpisah.',
  sandbox: 'Dunia terbuka tempat pemain membangun dan bereksperimen. Port sandbox di Android sering kali merupakan reimplementasi engine dengan dukungan mod dan penyimpanan dunia yang besar.',
  roguelike: 'Dungeon acak, permadeath, dan build karakter. Port roguelike native nyaman dimainkan singkat karena save cepat dan kontrol sederhana; banyak di antaranya open source.',
  rhythm: 'Permainan irama yang bergantung pada sinkronisasi audio. Port native rhythm perlu kalibrasi latensi audio di pengaturan; gunakan earphone kabel untuk hasil terbaik.',
  card: 'Permainan kartu seperti roguelike deckbuilder dan card battle. Port native kartu menonjolkan antarmuka sentuh besar dan sesi singkat, cocok dimainkan sambil santai.',
  board: 'Catur, dam, dan board game klasik. Port native board game biasanya ringan, mendukung multiplayer lokal, dan tidak membutuhkan perangkat kelas atas.',
  mmo: 'Dunia online persisten dengan banyak pemain. Status port MMO di Android bervariasi; periksa dukungan server, akun, dan koneksi stabil sebelum mencoba.'
};

const PLATFORM_INFO = {
  android: 'Android adalah target utama katalog ini. Semua port di halaman ini punya build yang bisa dipasang langsung sebagai APK, tanpa emulator dan tanpa ROM.',
  windows: 'Sebagian proyek port juga menyediakan build desktop Windows. Daftar ini menampilkan port yang memiliki versi Windows, berguna untuk membandingkan performa atau menyiapkan berkas data.',
  linux: 'Build Linux sering jadi rilis pertama sebuah source port. Halaman ini mengumpulkan port yang tersedia untuk Linux — banyak di antaranya open source dan bisa dikompilasi sendiri.',
  macos: 'Port dengan dukungan macOS biasanya datang dari proyek source port lintas platform. Cocok untuk memainkan versi desktop sambil menyiapkan data game untuk perangkat Android.',
  ios: 'Sebagian port punya build iOS selain Android. Perlu diingat pemasangan di iOS membutuhkan sideloading atau TestFlight, bukan APK.',
  'steam-deck': 'Steam Deck menjalankan build Linux dari banyak source port. Port yang tercantum di sini punya dukungan desktop Linux yang biasanya juga berjalan di SteamOS.',
  web: 'Build web memungkinkan game dimainkan langsung di browser. Beberapa source port menyediakan versi WebAssembly sebagai alternatif ringan sebelum memasang APK.',
  quest: 'Port untuk Meta Quest memakai kontrol VR atau gamepad. Halaman ini menandai port yang menyediakan build khusus headset.',
  switch: 'Beberapa source port dikompilasi untuk Nintendo Switch homebrew. Perlu CFW dan proses pemasangan terpisah; utamakan build Android bila tersedia.'
};

const KATEGORI_INFO = {
  'source-port': 'Source port adalah reimplementasi engine game dari kode sumber yang dirilis resmi. Pendekatan ini paling aman dan paling mudah dipelihara karena tidak menyentuh aset game asli; Anda tetap membutuhkan berkas data dari game yang Anda miliki.',
  decompilation: 'Port dekompilasi dibuat dengan mengembalikan kode dari binary game, lalu menyusunnya ulang untuk Android. Hasilnya mendekati versi asli, tetapi dukungan perangkat dan status hukumnya lebih rumit.',
  'n64-recomp': 'Rekompilasi N64 menerjemahkan kode MIPS Nintendo 64 menjadi kode native. Hasilnya jauh lebih ringan daripada emulator dan sering menambahkan widescreen serta peningkatan frame rate.',
  'nintendo-recomp': 'Kategori ini mencakup rekompilasi game Nintendo dari berbagai generasi. Perhatikan bahwa banyak proyek bersifat komunitas dan membutuhkan berkas ROM atau data game asli.',
  'xbox360-recomp': 'Rekompilasi Xbox 360 adalah proyek eksperimental yang menerjemahkan kode PowerPC ke ARM. Statusnya umumnya alpha atau eksperimental, jadi ekspektasi performa perlu dijaga.',
  'xbox-ps-psp-pc': 'Kategori ini memuat port dari Xbox, PlayStation, PSP, dan PC. Sebagian besar berupa source port atau reimplementasi engine yang sudah matang di desktop.',
  'handheld-16bit': 'Game handheld 16-bit dan generasi sebelumnya. Port native di kategori ini sering hadir sebagai reimplementasi engine atau runtime khusus yang ringan di perangkat modern.',
  'runtime-launcher': 'Runtime dan launcher adalah lapisan yang menjalankan beberapa game sekaligus, misalnya engine multi-game atau loader. Halaman ini membantu Anda menemukan basis yang dipakai banyak port.',
  classic: 'Port klasik dengan aktivitas pengembangan rendah tetap dipertahankan karena banyak dicari. Cek catatan instalasi dan kompatibilitas Android versi baru sebelum memasang.'
};

const LANDING = {
  genre:    { label: 'Genre',    info: GENRE_INFO,    judul: (n) => `Port Game ${n} untuk Android`,      h1: (n) => `Port game ${n} untuk Android` },
  platform: { label: 'Platform', info: PLATFORM_INFO, judul: (n) => `Port yang Tersedia di ${n}`,        h1: (n) => `Port native untuk ${n}` },
  kategori: { label: 'Kategori', info: KATEGORI_INFO, judul: (n) => `${n}: Port Native Android`,          h1: (n) => `Port ${n}` }
};

/* ═══════════════════════════════════════════════════════════════════════ */

async function fetchJson(url, timeoutMs = 15000) {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const res = await fetch(url, { signal: ctrl.signal, headers: { Accept: 'application/json' } });
    if (!res.ok) {
      let msg = `HTTP ${res.status}`;
      try { const b = await res.json(); if (b.error) msg = b.error; } catch (_) {}
      throw new Error(msg);
    }
    return await res.json();
  } finally { clearTimeout(timer); }
}

function setBadge(kind, text, title) {
  const b = $('#dbBadge');
  b.classList.remove('online','offline');
  if (kind) b.classList.add(kind);
  $('#dbText').textContent = text;
  b.title = title || '';
}

async function checkHealth() {
  try {
    const h = await fetchJson(API + '/health', 6000);
    setBadge('online', 'Online', `${h.user}@${h.database}`);
    $('#footInfo').textContent = 'GamePort';
  } catch (e) {
    setBadge('offline', 'Offline', e.message);
    $('#footInfo').textContent = 'layanan tidak tersedia';
  }
}

async function loadStats() {
  try {
    const s = await fetchJson(API + '/statistik');
    const genreList = await fetchJson(API + '/genre');
    const genreAktif = genreList.filter(g => g.jumlah > 0).length;
    const cells = [
      [s.terbit, 'Total Port'],
      [genreAktif, 'Genre'],
      [s.stabil, 'Stabil'],
      [s.total_download, 'Diunduh']
    ];
    $('#stats').innerHTML = cells.map(([n,l]) =>
      `<div class="stat"><div class="n">${Number(n||0).toLocaleString('id-ID')}</div><div class="l">${l}</div></div>`
    ).join('');
  } catch (_) {}
}

async function loadFilters() {
  let cats, plats, gens;
  try {
    [cats, plats, gens] = await Promise.all([
      fetchJson(API + '/kategori'), fetchJson(API + '/platform'), fetchJson(API + '/genre')
    ]);
  } catch (_) { return; }

  const aktif = cats.filter(c => c.jumlah > 0);
  $('#catChips').innerHTML =
    '<button class="chip active" data-cat="">Semua kategori</button>' +
    aktif.map(c => `<button class="chip" data-cat="${esc(c.slug)}">${esc(c.name)}<span class="c">${c.jumlah}</span></button>`).join('');

  $('#genreSel').innerHTML = '<option value="">Semua genre</option>' +
    gens.filter(g => g.jumlah > 0).map(g => `<option value="${esc(g.slug)}">${esc(g.name)} (${g.jumlah})</option>`).join('');

  $('#platformSel').innerHTML = '<option value="">Semua platform</option>' +
    plats.filter(p => p.jumlah > 0).map(p => `<option value="${esc(p.slug)}">${esc(p.name)} (${p.jumlah})</option>`).join('');
}

/* Menyelaraskan kontrol dengan state yang datang dari URL. */
function terapkanStateKeKontrol() {
  $('#q').value = state.q;
  $('#genreSel').value = state.genre;
  $('#platformSel').value = state.platform;
  $('#sortSel').value = state.sort;
  document.querySelectorAll('#catChips .chip').forEach(b => {
    b.classList.toggle('active', (b.dataset.cat || '') === state.kategori);
  });
}

/* Simpan filter ke URL agar bisa dibagikan dan tombol kembali bekerja. */
function tulisStateKeURL() {
  const u = new URLSearchParams();
  if (state.q) u.set('q', state.q);
  if (state.kategori) u.set('kategori', state.kategori);
  if (state.genre) u.set('genre', state.genre);
  if (state.platform) u.set('platform', state.platform);
  if (state.sort && state.sort !== 'terbaru') u.set('sort', state.sort);
  const qs = u.toString();
  history.replaceState(null, '', qs ? `/?${qs}` : '/');
}

function bacaStateDariURL() {
  const u = new URLSearchParams(location.search);
  if (u.get('q')) state.q = u.get('q');
  if (u.get('kategori')) state.kategori = u.get('kategori');
  if (u.get('genre')) state.genre = u.get('genre');
  if (u.get('platform')) state.platform = u.get('platform');
  if (u.get('sort')) state.sort = u.get('sort');
}

function badgeHtml(slug, name) {
  const st = STATUS_STYLE[slug] || { bg:'rgba(255,255,255,.75)', fg:'#111' };
  return `<span class="badge" style="background:${st.bg};color:${st.fg}">${esc(name)}</span>`;
}

function thumbHtml(p) {
  if (p.image_url) {
    return `<div class="thumb"><img src="${esc(asetAPI(p.image_url))}" alt="${esc(p.game)} — port Android" loading="lazy" width="640" height="360"
              onerror="this.parentElement.innerHTML='<div class=\\'ph\\'>${esc((p.game||'?')[0])}</div>'">
              ${badgeHtml(p.status_slug, p.status_name)}</div>`;
  }
  return `<div class="thumb"><div class="ph">${esc((p.game||'?').slice(0,2).toUpperCase())}</div>
          ${badgeHtml(p.status_slug, p.status_name)}</div>`;
}

/* Tautan internal ke landing page — penting untuk penemuan halaman oleh crawler. */
const genreUrl = (p) => p.genre_slug ? `/genre/${encodeURIComponent(p.genre_slug)}` : '';
const kategoriUrl = (p) => `/kategori/${encodeURIComponent(p.category_slug)}`;
const platformUrl = (x) => `/platform/${encodeURIComponent(x)}`;

function cardHtml(p) {
  const platforms = (p.platforms||[])
    .map(x => `<a class="tag lnk" href="${platformUrl(x)}">${esc(PLATFORM_LABEL[x]||x)}</a>`).join('');
  const meta = [];
  if (p.version) meta.push(`<span class="mi">${esc(verLabel(p.version))}</span>`);
  if (p.file_size) meta.push(`<span class="mi">${esc(p.file_size)}</span>`);
  if (p.genre_name && p.genre_slug) meta.push(`<a class="mi g lnk" href="${genreUrl(p)}">${esc(p.genre_name)}</a>`);

  const url = `/port/${encodeURIComponent(p.slug)}`;

  return `
  <article class="card" data-slug="${esc(p.slug)}">
    <a class="card-hit" href="${url}" aria-label="${esc(p.game)}"></a>
    ${thumbHtml(p)}
    <div class="body">
      <span class="cat"><a class="lnk" href="${kategoriUrl(p)}">${esc(p.category_name)}</a></span>
      <h3><a href="${url}">${esc(p.game)}</a></h3>
      <div class="metarow">${meta.join('')}</div>
      <p>${esc(p.description) || '<em style="color:var(--fg-3)">tanpa deskripsi</em>'}</p>
      <div class="tags">${platforms}</div>
      <div class="card-foot">
        <span class="stat-mini"><span>👁 ${Number(p.view_count||0).toLocaleString('id-ID')}</span>
        <span>↓ ${Number(p.download_count||0).toLocaleString('id-ID')}</span></span>
        <span class="card-more">Detail →</span>
      </div>
    </div>
  </article>`;
}

let seq = 0;
async function loadPorts() {
  const my = ++seq;
  $('#content').innerHTML = '<div class="state"><div class="spinner"></div><div class="big">Mengambil data…</div></div>';

  const p = new URLSearchParams({ limit: state.limit, offset: state.offset, sort: state.sort });
  if (state.q) p.set('q', state.q);
  if (state.kategori) p.set('kategori', state.kategori);
  if (state.genre) p.set('genre', state.genre);
  if (state.platform) p.set('platform', state.platform);

  let d;
  try { d = await fetchJson(API + '/ports?' + p.toString()); }
  catch (e) {
    if (my !== seq) return;
    $('#resultInfo').textContent = 'gagal memuat';
    $('#content').innerHTML = `<div class="state"><div class="big">Gagal mengambil data.</div>
      <div class="mono" style="color:var(--bad)">${esc(e.message)}</div></div>`;
    $('#pager').hidden = true; return;
  }
  if (my !== seq) return;

  $('#resultInfo').textContent = `${d.total.toLocaleString('id-ID')} port ditemukan`;
  const aktif = [state.q && `"${state.q}"`, state.kategori, state.genre, state.platform].filter(Boolean);
  $('#queryInfo').textContent = aktif.length ? `filter: ${aktif.join(' · ')}` : '';

  if (!d.items.length) {
    $('#content').innerHTML = `<div class="state"><div class="big">Belum ada port yang cocok.</div>
      <div>Coba ubah kata kunci atau reset filter.</div></div>`;
    $('#pager').hidden = true; return;
  }

  $('#content').innerHTML = `<div class="grid">${d.items.map(cardHtml).join('')}</div>`;
  const from = d.offset + 1, to = Math.min(d.offset + d.limit, d.total);
  $('#pageInfo').textContent = `${from}–${to} / ${d.total}`;
  $('#prev').disabled = d.offset <= 0;
  $('#next').disabled = to >= d.total;
  $('#pager').hidden = d.total <= d.limit;
}

let t;
$('#q').addEventListener('input', e => {
  clearTimeout(t);
  t = setTimeout(() => { state.q = e.target.value.trim(); state.offset = 0; tulisStateKeURL(); loadPorts(); }, 280);
});
$('#genreSel').addEventListener('change', e => { state.genre = e.target.value; state.offset = 0; tulisStateKeURL(); loadPorts(); });
$('#platformSel').addEventListener('change', e => { state.platform = e.target.value; state.offset = 0; tulisStateKeURL(); loadPorts(); });
$('#sortSel').addEventListener('change', e => { state.sort = e.target.value; state.offset = 0; tulisStateKeURL(); loadPorts(); });
$('#prev').addEventListener('click', () => { state.offset = Math.max(0, state.offset - state.limit); loadPorts(); window.scrollTo({top:400,behavior:'smooth'}); });
$('#next').addEventListener('click', () => { state.offset += state.limit; loadPorts(); window.scrollTo({top:400,behavior:'smooth'}); });
$('#catChips').addEventListener('click', e => {
  const b = e.target.closest('.chip'); if (!b) return;
  document.querySelectorAll('#catChips .chip').forEach(x => x.classList.remove('active'));
  b.classList.add('active');
  state.kategori = b.dataset.cat; state.offset = 0; tulisStateKeURL(); loadPorts();
});

/* ═════════════════════ landing page genre/platform/kategori ═════════════════════ */

function tautanSaudara(kind, daftar, aktif) {
  return daftar
    .filter(x => x.jumlah > 0)
    .map(x => `<a class="cloud-link${x.slug === aktif ? ' on' : ''}" href="/${kind}/${encodeURIComponent(x.slug)}">${esc(x.name)}<span>${x.jumlah}</span></a>`)
    .join('');
}

function landingHtml(kind, ref, items, saudara) {
  const meta = LANDING[kind];
  const judul = meta.h1(ref.name);
  const grid = items.length
    ? `<div class="grid">${items.map(cardHtml).join('')}</div>`
    : `<div class="state"><div class="big">Belum ada port di kategori ini.</div>
         <div><a href="/">Kembali ke katalog</a></div></div>`;

  return `
  <nav class="crumbs"><a href="/">Katalog</a><span>›</span><span class="now">${esc(ref.name)}</span></nav>
  <header class="landing-head">
    <div class="eyebrow">${meta.label} · ${ref.jumlah} port</div>
    <h1>${esc(judul)}</h1>
    <p class="lede">${esc(meta.info[ref.slug] || '')}</p>
  </header>
  <div class="meta"><span>${items.length.toLocaleString('id-ID')} port ditampilkan</span>
    <span class="mono">${meta.label}: ${esc(ref.name)}</span></div>
  ${grid}
  <section class="landing-links">
    <h2>Jelajahi ${meta.label.toLowerCase()} lain</h2>
    <div class="link-cloud">${tautanSaudara(kind, saudara, ref.slug)}</div>
  </section>
  <p class="back-all"><a href="/">← Lihat seluruh katalog port native</a></p>`;
}

async function loadLanding(kind, slug) {
  $('#homeView').hidden = true;
  $('#landingView').hidden = false;
  const tentang = $('#tentang');
  if (tentang) tentang.hidden = true;
  $('#landingContent').innerHTML =
    '<div class="state"><div class="spinner"></div><div class="big">Memuat…</div></div>';

  const meta = LANDING[kind];
  let daftar, data;
  try {
    const filter = kind === 'genre' ? 'genre' : kind === 'platform' ? 'platform' : 'kategori';
    const refUrl = kind === 'genre' ? '/genre' : kind === 'platform' ? '/platform' : '/kategori';
    [daftar, data] = await Promise.all([
      fetchJson(API + refUrl),
      fetchJson(`${API}/ports?${filter}=${encodeURIComponent(slug)}&limit=60&sort=populer`)
    ]);
  } catch (e) {
    applySeo({
      title: 'Gagal memuat katalog — GamePort', path: location.pathname,
      robots: 'noindex, follow'
    });
    $('#landingContent').innerHTML =
      `<div class="state"><div class="big">Gagal memuat katalog.</div>
       <div class="mono" style="color:var(--bad)">${esc(e.message)}</div></div>`;
    return;
  }

  const ref = daftar.find(x => x.slug === slug && x.jumlah > 0);
  if (!ref) {
    applySeo({
      title: 'Halaman tidak ditemukan — GamePort', path: location.pathname,
      robots: 'noindex, follow'
    });
    $('#landingContent').innerHTML =
      `<div class="state" style="padding-top:60px">
         <div class="big">Halaman tidak ditemukan.</div>
         <div style="margin-bottom:15px">Kategori ${esc(slug)} tidak ada di katalog.</div>
         <a class="btn primary" href="/" style="display:inline-flex;flex-direction:row"><span>← Kembali ke katalog</span></a>
       </div>`;
    return;
  }

  const judul = meta.h1(ref.name);
  const intro = meta.info[ref.slug] || `Kumpulan port native Android untuk ${ref.name}.`;
  const path = `/${kind}/${encodeURIComponent(ref.slug)}`;
  const url = SITE + path;

  applySeo({
    title: `${meta.judul(ref.name)} — GamePort`,
    description: ringkas(intro, 157),
    path,
    type: 'website',
    jsonLd: {
      '@context': 'https://schema.org',
      '@graph': [
        {
          '@type': 'CollectionPage',
          '@id': url + '#halaman',
          'url': url,
          'name': meta.judul(ref.name),
          'description': ringkas(intro, 300),
          'inLanguage': 'id-ID',
          'isPartOf': { '@id': SITE + '/#website' }
        },
        {
          '@type': 'BreadcrumbList',
          'itemListElement': [
            { '@type': 'ListItem', 'position': 1, 'name': 'Katalog', 'item': SITE + '/' },
            { '@type': 'ListItem', 'position': 2, 'name': ref.name, 'item': url }
          ]
        },
        {
          '@type': 'ItemList',
          'itemListElement': data.items.slice(0, 30).map((p, i) => ({
            '@type': 'ListItem',
            'position': i + 1,
            'url': SITE + '/port/' + encodeURIComponent(p.slug),
            'name': p.game
          }))
        }
      ]
    }
  });

  $('#landingContent').innerHTML = landingHtml(kind, ref, data.items, daftar);
  window.scrollTo({ top: 0, behavior: 'instant' });
}

/* ═════════════════════ bagian bersama halaman detail ═════════════════════ */

function shareHtml(p) {
  const url = SITE + '/port/' + encodeURIComponent(p.slug);
  const teks = `${p.game} — port native Android di GamePort`;
  const wa = 'https://wa.me/?text=' + encodeURIComponent(`${teks} ${url}`);
  const tg = 'https://t.me/share/url?url=' + encodeURIComponent(url) + '&text=' + encodeURIComponent(teks);
  const x  = 'https://twitter.com/intent/tweet?url=' + encodeURIComponent(url) + '&text=' + encodeURIComponent(teks);
  const fb = 'https://www.facebook.com/sharer/sharer.php?u=' + encodeURIComponent(url);
  return `
  <div class="share">
    <span class="share-label">Bagikan</span>
    <a href="${wa}" target="_blank" rel="noopener nofollow" aria-label="Bagikan ke WhatsApp">WhatsApp</a>
    <a href="${tg}" target="_blank" rel="noopener nofollow" aria-label="Bagikan ke Telegram">Telegram</a>
    <a href="${x}" target="_blank" rel="noopener nofollow" aria-label="Bagikan ke X">X</a>
    <a href="${fb}" target="_blank" rel="noopener nofollow" aria-label="Bagikan ke Facebook">Facebook</a>
    <button type="button" data-copy>Salin tautan</button>
  </div>`;
}

function pasangShare(root) {
  const copy = root.querySelector('[data-copy]');
  if (!copy) return;
  copy.addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(location.href);
      copy.textContent = 'Tersalin ✓';
      setTimeout(() => { copy.textContent = 'Salin tautan'; }, 1800);
    } catch (_) {
      copy.textContent = 'Gagal menyalin';
    }
  });
}

function detailHtml(p) {
  const plats = (p.platforms||[])
    .map(x => `<a class="tag lnk" href="${platformUrl(x)}">${esc(PLATFORM_LABEL[x]||x)}</a>`).join('');
  const meta = [
    p.version   ? esc(verLabel(p.version)) : '',
    p.file_size ? esc(p.file_size) : '',
    p.genre_name && p.genre_slug
      ? `<a class="lnk" href="${genreUrl(p)}">${esc(p.genre_name)}</a>` : ''
  ].filter(Boolean).map(x => `<span class="mi">${x}</span>`).join('');

  const initials = esc((p.game||'?').slice(0,2).toUpperCase());
  const cover = p.image_url
    ? `<img src="${esc(asetAPI(p.image_url))}" alt="${esc(p.game)} — port native Android" width="640" height="360"
         onerror="this.parentElement.insertAdjacentHTML('afterbegin','<div class=&quot;ph&quot;>${initials}</div>');this.remove()">`
    : `<div class="ph">${initials}</div>`;

  const dlBase = API_ORIGIN + '/dl/' + encodeURIComponent(p.slug);
  const btnPort = p.download_url
    ? `<a class="btn primary" href="${dlBase}/port" rel="nofollow">
         <span>Unduh Port</span><small>langsung jalan di Android</small></a>`
    : `<span class="btn disabled"><span>Port belum tersedia</span><small>penyedia belum mengunggah</small></span>`;

  const btnIso = p.iso_url
    ? `<a class="btn ghost" href="${dlBase}/iso" rel="nofollow">
         <span>Unduh ISO / Game</span><small>file game asli</small></a>`
    : '';

  const related = (p.related && p.related.length)
    ? `<section class="detail-related"><h2>Port terkait</h2>
         <div class="grid">${p.related.map(cardHtml).join('')}</div></section>`
    : '';

  let srcHost = '';
  if (p.source_url) {
    try { srcHost = new URL(p.source_url).hostname.replace(/^www\./, ''); }
    catch (_) { srcHost = 'sumber'; }
  }

  return `
  <nav class="crumbs"><a href="/">Katalog</a><span>›</span>
    <a href="${kategoriUrl(p)}">${esc(p.category_name)}</a><span>›</span>
    <span class="now">${esc(p.game)}</span></nav>
  <article class="detail">
    <div class="detail-top">
      <div class="detail-cover">${cover}${badgeHtml(p.status_slug, p.status_name)}</div>
      <div class="detail-info">
        <span class="cat"><a class="lnk" href="${kategoriUrl(p)}">${esc(p.category_name)}</a></span>
        <h1>${esc(p.game)}</h1>
        <div class="metarow">${meta}</div>
        <div class="tags">${plats}</div>
        <div class="detail-stats">
          <div><b>${Number(p.view_count||0).toLocaleString('id-ID')}</b><span>dilihat</span></div>
          <div><b>${Number(p.download_count||0).toLocaleString('id-ID')}</b><span>diunduh</span></div>
          ${p.needs_own_data ? '<div><b>Ya</b><span>butuh data asli</span></div>' : ''}
        </div>
        <div class="detail-actions">${btnPort}${btnIso}</div>
        ${p.author ? `<div class="author">Porter: <strong>${esc(p.author)}</strong></div>` : ''}
        ${p.source_url ? `<div class="author">Sumber: <a href="${esc(p.source_url)}" target="_blank" rel="noopener nofollow">${esc(srcHost)}</a></div>` : ''}
        ${shareHtml(p)}
      </div>
    </div>
    <section class="detail-desc">
      <h2>Deskripsi</h2>
      <p>${esc(p.description) || '<em style="color:var(--fg-3)">Belum ada deskripsi.</em>'}</p>
    </section>
    ${p.install_notes ? `<section class="detail-notes">
      <h2>Cara Instalasi</h2>
      <p>${esc(p.install_notes)}</p>
    </section>` : ''}
    ${related}
  </article>`;
}

function detailJsonLd(p, url, img, desc) {
  const app = {
    '@type': 'SoftwareApplication',
    'name': p.game,
    'url': url,
    'applicationCategory': 'GameApplication',
    'operatingSystem': 'Android',
    'inLanguage': 'id-ID',
    'description': desc,
    'offers': { '@type': 'Offer', 'price': '0', 'priceCurrency': 'IDR' },
    'isPartOf': { '@id': SITE + '/#website' }
  };
  if (p.version) app.softwareVersion = p.version;
  if (p.file_size) app.fileSize = p.file_size;
  if (img) app.image = img;
  if (p.author) app.author = { '@type': 'Person', 'name': p.author };
  if (p.genre_name) app.genre = p.genre_name;

  return {
    '@context': 'https://schema.org',
    '@graph': [
      app,
      {
        '@type': 'BreadcrumbList',
        'itemListElement': [
          { '@type': 'ListItem', 'position': 1, 'name': 'Katalog', 'item': SITE + '/' },
          { '@type': 'ListItem', 'position': 2, 'name': p.category_name, 'item': SITE + kategoriUrl(p) },
          { '@type': 'ListItem', 'position': 3, 'name': p.game, 'item': url }
        ]
      }
    ]
  };
}

async function loadDetail(slug) {
  $('#homeView').hidden = true;
  $('#detailView').hidden = false;
  const tentang = $('#tentang');
  if (tentang) tentang.hidden = true;
  $('#detailContent').innerHTML =
    '<div class="state"><div class="spinner"></div><div class="big">Memuat detail…</div></div>';

  let p;
  try { p = await fetchJson(API + '/ports/' + encodeURIComponent(slug)); }
  catch (e) {
    applySeo({
      title: 'Port tidak ditemukan — GamePort',
      description: 'Port yang Anda cari tidak ada di katalog GamePort.',
      path: location.pathname,
      robots: 'noindex, follow'
    });
    $('#detailContent').innerHTML = `
      <div class="state" style="padding-top:60px">
        <div class="big">Port tidak ditemukan.</div>
        <div style="margin-bottom:15px">${esc(e.message)}</div>
        <a class="btn primary" href="/" style="display:inline-flex;flex-direction:row"><span>← Kembali ke katalog</span></a>
      </div>`;
    return;
  }

  const path = '/port/' + encodeURIComponent(p.slug);
  const url = SITE + path;
  const img = p.image_url ? asetAPI(p.image_url) : '';
  const desc = ringkas(
    p.description ||
    `Unduh port native ${p.game} untuk Android${p.author ? ' oleh ' + p.author : ''}. ${p.category_name}, status ${p.status_name}.`,
    157
  );

  applySeo({
    title: `${p.game} — Port Android${p.version ? ' v' + p.version : ''} | GamePort`,
    description: desc,
    path,
    image: img || undefined,
    type: 'article',
    jsonLd: detailJsonLd(p, url, img, desc)
  });

  $('#detailContent').innerHTML = detailHtml(p);
  pasangShare($('#detailContent'));
  window.scrollTo({ top: 0, behavior: 'instant' });
}

// klik kartu → halaman detail (kecuali menekan tautan/tombol unduh)
function pasangKlikKartu(root) {
  root.addEventListener('click', (e) => {
    if (e.target.closest('[data-nolink]')) return;
    if (e.target.closest('a.lnk')) return;            // biarkan tautan kategori/genre/platform
    const card = e.target.closest('.card[data-slug]');
    if (!card) return;
    if (e.metaKey || e.ctrlKey || e.shiftKey) return;   // biarkan buka tab baru
    window.location.href = '/port/' + encodeURIComponent(card.dataset.slug);
  });
}

const detailRoute  = location.pathname.match(/^\/port\/([^/]+)\/?$/);
const landingRoute = location.pathname.match(/^\/(genre|platform|kategori)\/([^/]+)\/?$/);

(async function init() {
  checkHealth();

  if (detailRoute) {
    await loadDetail(decodeURIComponent(detailRoute[1]));
    setInterval(checkHealth, 60000);
    return;
  }

  if (landingRoute) {
    await loadLanding(landingRoute[1], decodeURIComponent(landingRoute[2]));
    setInterval(checkHealth, 60000);
    return;
  }

  bacaStateDariURL();
  await Promise.all([loadStats(), loadFilters(), loadPorts()]);
  terapkanStateKeKontrol();
  pasangKlikKartu($('#content'));
  pasangKlikKartu($('#detailContent'));
  setInterval(checkHealth, 60000);
})();
