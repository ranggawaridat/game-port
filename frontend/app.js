/* GamePort — frontend publik. Semua data dari API. */
'use strict';

const state = { q:'', kategori:'', status:'', platform:'', genre:'', sort:'terbaru', limit:24, offset:0 };
const $ = (s) => document.querySelector(s);
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

/* ---------- Konfigurasi origin ----------
 * Frontend dipisah dari backend:
 *   publik  : https://gameport.web.id          (Vercel, statis)
 *   backend : https://api.gameport.web.id/v1   (VPS, Go)
 *
 * Bila halaman ini dilayani oleh backend sendiri (dev / masa monolitik),
 * base dibiarkan kosong sehingga semua URL tetap relative.
 */
const API_ORIGIN = 'https://api.gameport.web.id';
const API = `${API_ORIGIN}/api/v1`;

/* Gambar sampul bisa URL eksternal (http/https) atau path lama /uploads/…
   yang masih dilayani backend. Path relative harus diawali origin API. */
const asetAPI = (u) => (u && u.startsWith('/') ? API_ORIGIN + u : u);

const STATUS_STYLE = {
  stable:       { bg:'rgba(34,197,94,.9)',  fg:'#04140a' },
  beta:         { bg:'rgba(245,158,11,.9)', fg:'#1a1003' },
  experimental: { bg:'rgba(234,88,12,.9)',  fg:'#180a02' },
  alpha:        { bg:'rgba(239,68,68,.9)',  fg:'#170404' }
};
const PLATFORM_LABEL = { android:'Android', windows:'Windows', linux:'Linux', macos:'macOS',
  ios:'iOS','steam-deck':'Steam Deck', web:'Web', quest:'Quest', switch:'Switch' };

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

function badgeHtml(slug, name) {
  const st = STATUS_STYLE[slug] || { bg:'rgba(255,255,255,.75)', fg:'#111' };
  return `<span class="badge" style="background:${st.bg};color:${st.fg}">${esc(name)}</span>`;
}

function thumbHtml(p) {
  if (p.image_url) {
    return `<div class="thumb"><img src="${esc(asetAPI(p.image_url))}" alt="${esc(p.game)}" loading="lazy"
              onerror="this.parentElement.innerHTML='<div class=\\'ph\\'>${esc((p.game||'?')[0])}</div>'">
              ${badgeHtml(p.status_slug, p.status_name)}</div>`;
  }
  return `<div class="thumb"><div class="ph">${esc((p.game||'?').slice(0,2).toUpperCase())}</div>
          ${badgeHtml(p.status_slug, p.status_name)}</div>`;
}

function cardHtml(p) {
  const platforms = (p.platforms||[]).map(x => `<span class="tag">${esc(PLATFORM_LABEL[x]||x)}</span>`).join('');
  const meta = [];
  if (p.version) meta.push(`<span class="mi">v${esc(p.version)}</span>`);
  if (p.file_size) meta.push(`<span class="mi">${esc(p.file_size)}</span>`);
  if (p.genre_name) meta.push(`<span class="mi g">${esc(p.genre_name)}</span>`);

  const url = `/port/${encodeURIComponent(p.slug)}`;

  return `
  <article class="card" data-slug="${esc(p.slug)}">
    <a class="card-hit" href="${url}" aria-label="${esc(p.game)}"></a>
    ${thumbHtml(p)}
    <div class="body">
      <span class="cat">${esc(p.category_name)}</span>
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
  t = setTimeout(() => { state.q = e.target.value.trim(); state.offset = 0; loadPorts(); }, 280);
});
$('#genreSel').addEventListener('change', e => { state.genre = e.target.value; state.offset = 0; loadPorts(); });
$('#platformSel').addEventListener('change', e => { state.platform = e.target.value; state.offset = 0; loadPorts(); });
$('#sortSel').addEventListener('change', e => { state.sort = e.target.value; state.offset = 0; loadPorts(); });
$('#prev').addEventListener('click', () => { state.offset = Math.max(0, state.offset - state.limit); loadPorts(); window.scrollTo({top:400,behavior:'smooth'}); });
$('#next').addEventListener('click', () => { state.offset += state.limit; loadPorts(); window.scrollTo({top:400,behavior:'smooth'}); });
$('#catChips').addEventListener('click', e => {
  const b = e.target.closest('.chip'); if (!b) return;
  document.querySelectorAll('#catChips .chip').forEach(x => x.classList.remove('active'));
  b.classList.add('active');
  state.kategori = b.dataset.cat; state.offset = 0; loadPorts();
});

// ───────────────────────── halaman detail ─────────────────────────
function detailHtml(p) {
  const plats = (p.platforms||[])
    .map(x => `<span class="tag">${esc(PLATFORM_LABEL[x]||x)}</span>`).join('');
  const meta = [
    p.version   ? `v${esc(p.version)}` : '',
    p.file_size ? esc(p.file_size) : '',
    p.genre_name? esc(p.genre_name) : ''
  ].filter(Boolean).map(x => `<span class="mi">${x}</span>`).join('');

  const initials = esc((p.game||'?').slice(0,2).toUpperCase());
  const cover = p.image_url
    ? `<img src="${esc(asetAPI(p.image_url))}" alt="${esc(p.game)}"
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
  <nav class="crumbs"><a href="/">Katalog</a><span>›</span><span class="now">${esc(p.game)}</span></nav>
  <article class="detail">
    <div class="detail-top">
      <div class="detail-cover">${cover}${badgeHtml(p.status_slug, p.status_name)}</div>
      <div class="detail-info">
        <span class="cat">${esc(p.category_name)}</span>
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

async function loadDetail(slug) {
  $('#homeView').hidden = true;
  $('#detailView').hidden = false;
  $('#detailContent').innerHTML =
    '<div class="state"><div class="spinner"></div><div class="big">Memuat detail…</div></div>';

  let p;
  try { p = await fetchJson(API + '/ports/' + encodeURIComponent(slug)); }
  catch (e) {
    document.title = 'Tidak ditemukan — GamePort';
    $('#detailContent').innerHTML = `
      <div class="state" style="padding-top:60px">
        <div class="big">Port tidak ditemukan.</div>
        <div style="margin-bottom:15px">${esc(e.message)}</div>
        <a class="btn primary" href="/" style="display:inline-flex;flex-direction:row"><span>← Kembali ke katalog</span></a>
      </div>`;
    return;
  }
  document.title = `${p.game} — GamePort`;
  $('#detailContent').innerHTML = detailHtml(p);
  window.scrollTo({ top: 0, behavior: 'instant' });
}

// klik kartu → halaman detail (kecuali menekan tombol unduh)
function pasangKlikKartu(root) {
  root.addEventListener('click', (e) => {
    if (e.target.closest('[data-nolink]')) return;
    const card = e.target.closest('.card[data-slug]');
    if (!card) return;
    if (e.metaKey || e.ctrlKey || e.shiftKey) return;   // biarkan buka tab baru
    window.location.href = '/port/' + encodeURIComponent(card.dataset.slug);
  });
}

const detailRoute = location.pathname.match(/^\/port\/([^/]+)\/?$/);

(async function init() {
  await checkHealth();

  if (detailRoute) {
    await loadDetail(decodeURIComponent(detailRoute[1]));
    setInterval(checkHealth, 60000);
    return;
  }

  await Promise.all([loadStats(), loadFilters()]);
  await loadPorts();
  pasangKlikKartu($('#content'));
  pasangKlikKartu($('#detailContent'));
  setInterval(checkHealth, 60000);
})();