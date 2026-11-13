/* GamePort — frontend publik. Semua data dari API. */
'use strict';

const state = { q:'', kategori:'', status:'', platform:'', genre:'', sort:'terbaru', limit:24, offset:0 };
const $ = (s) => document.querySelector(s);
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

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
    const h = await fetchJson('/api/health', 6000);
    setBadge('online', 'Online', `${h.user}@${h.database}`);
    $('#footInfo').textContent = 'GamePort';
  } catch (e) {
    setBadge('offline', 'Offline', e.message);
    $('#footInfo').textContent = 'layanan tidak tersedia';
  }
}

async function loadStats() {
  try {
    const s = await fetchJson('/api/statistik');
    const genreList = await fetchJson('/api/genre');
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
      fetchJson('/api/kategori'), fetchJson('/api/platform'), fetchJson('/api/genre')
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
    return `<div class="thumb"><img src="${esc(p.image_url)}" alt="${esc(p.game)}" loading="lazy"
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

  const dl = p.download_url
    ? `<a class="dl" href="/api/ports/${esc(p.slug)}/download" rel="nofollow">Unduh ↓</a>`
    : `<span class="dl off">Belum tersedia</span>`;

  return `
  <article class="card">
    ${thumbHtml(p)}
    <div class="body">
      <span class="cat">${esc(p.category_name)}</span>
      <h3>${esc(p.game)}</h3>
      <div class="metarow">${meta.join('')}</div>
      <p>${esc(p.description) || '<em style="color:var(--fg-3)">tanpa deskripsi</em>'}</p>
      <div class="tags">${platforms}</div>
      <div class="card-foot">
        <span class="stat-mini"><span>👁 ${Number(p.view_count||0).toLocaleString('id-ID')}</span>
        <span>↓ ${Number(p.download_count||0).toLocaleString('id-ID')}</span></span>
        ${dl}
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
  try { d = await fetchJson('/api/ports?' + p.toString()); }
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

(async function init() {
  await checkHealth();
  await Promise.all([loadStats(), loadFilters()]);
  await loadPorts();
  setInterval(checkHealth, 60000);
})();