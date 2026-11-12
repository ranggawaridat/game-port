/* =========================================================================
   GamePort — frontend vanilla JS.
   Semua data berasal dari API backend Go (yang membaca PostgreSQL).
   Tidak ada data yang ditulis mati di file ini.
   ========================================================================= */
'use strict';

const state = {
  q: '', kategori: '', status: '', platform: '',
  sort: 'kategori', limit: 24, offset: 0
};

const $ = (s) => document.querySelector(s);

const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({
  '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
}[c]));

const STATUS_STYLE = {
  stable:       { bg: 'rgba(34,197,94,.12)',  fg: '#4ade80', bd: 'rgba(34,197,94,.3)' },
  beta:         { bg: 'rgba(245,158,11,.12)', fg: '#fbbf24', bd: 'rgba(245,158,11,.3)' },
  experimental: { bg: 'rgba(234,88,12,.12)',  fg: '#fb923c', bd: 'rgba(234,88,12,.3)' },
  alpha:        { bg: 'rgba(239,68,68,.12)',  fg: '#f87171', bd: 'rgba(239,68,68,.3)' }
};

const PLATFORM_LABEL = {
  android: 'Android', windows: 'Windows', linux: 'Linux', macos: 'macOS',
  ios: 'iOS', 'steam-deck': 'Steam Deck', web: 'Web', quest: 'Quest', switch: 'Switch'
};

/* ---------- fetch dengan timeout ---------- */
async function fetchJson(url, timeoutMs = 15000) {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const res = await fetch(url, { signal: ctrl.signal, headers: { Accept: 'application/json' } });
    if (!res.ok) {
      let msg = `HTTP ${res.status}`;
      try {
        const body = await res.json();
        if (body.error) msg = body.error;
      } catch (_) { /* respons bukan JSON */ }
      throw new Error(msg);
    }
    return await res.json();
  } finally {
    clearTimeout(timer);
  }
}

/* ---------- badge status koneksi ---------- */
function setBadge(kind, text, title) {
  const badge = $('#dbBadge');
  badge.classList.remove('online', 'offline');
  if (kind) badge.classList.add(kind);
  $('#dbText').textContent = text;
  badge.title = title || '';
}

async function checkHealth() {
  try {
    const h = await fetchJson('/api/health', 6000);
    setBadge('online', 'DB online', `${h.user}@${h.database} · pool ${h.pool_total}`);
    $('#footInfo').textContent = `${h.user}@${h.database} · PostgreSQL`;
  } catch (err) {
    setBadge('offline', 'DB offline', err.message);
    $('#footInfo').textContent = 'database tidak terjangkau';
  }
}

/* ---------- statistik ---------- */
async function loadStats() {
  let s;
  try {
    s = await fetchJson('/api/statistik');
  } catch (_) {
    return;
  }
  const cells = [
    [s.total_port, 'Total Port'],
    [s.total_author, 'Author'],
    [s.stabil, 'Stabil'],
    [s.butuh_data_asli, 'Butuh Data Asli']
  ];
  $('#stats').innerHTML = cells.map(([n, l]) =>
    `<div class="stat"><div class="n">${Number(n || 0).toLocaleString('id-ID')}</div><div class="l">${l}</div></div>`
  ).join('');
}

/* ---------- filter dropdown & chips ---------- */
async function loadFilters() {
  let cats, stats, plats;
  try {
    [cats, stats, plats] = await Promise.all([
      fetchJson('/api/kategori'), fetchJson('/api/status'), fetchJson('/api/platform')
    ]);
  } catch (_) {
    return;
  }

  $('#catChips').innerHTML =
    '<button class="chip active" data-cat="">Semua kategori</button>' +
    cats.map((c) =>
      `<button class="chip" data-cat="${esc(c.slug)}">${esc(c.name)}<span class="c">${c.jumlah}</span></button>`
    ).join('');

  $('#statusSel').innerHTML = '<option value="">Semua status</option>' +
    stats.map((s) => `<option value="${esc(s.slug)}">${esc(s.name)} (${s.jumlah})</option>`).join('');

  $('#platformSel').innerHTML = '<option value="">Semua platform</option>' +
    plats.map((p) => `<option value="${esc(p.slug)}">${esc(p.name)} (${p.jumlah})</option>`).join('');
}

/* ---------- kartu ---------- */
function badgeHtml(slug, name) {
  const st = STATUS_STYLE[slug] || { bg: 'rgba(255,255,255,.06)', fg: 'var(--fg-2)', bd: 'var(--line)' };
  return `<span class="badge" style="background:${st.bg};color:${st.fg};border:1px solid ${st.bd}">${esc(name)}</span>`;
}

function cardHtml(p) {
  const platforms = (p.platforms || [])
    .map((x) => `<span class="tag">${esc(PLATFORM_LABEL[x] || x)}</span>`).join('');
  const engines = (p.engines || []).slice(0, 2)
    .map((x) => `<span class="tag">${esc(x)}</span>`).join('');
  const own = p.needs_own_data ? '<span class="tag own">butuh data asli</span>' : '';

  return `
  <article class="card">
    <div class="card-top">
      <span class="cat">${esc(p.category_name)}</span>
      ${badgeHtml(p.status_slug, p.status_name)}
    </div>
    <h3>${esc(p.game)}</h3>
    <p>${esc(p.description) || '<em style="color:var(--fg-3)">tanpa deskripsi</em>'}</p>
    <div class="tags">${platforms}${engines}${own}</div>
    <div class="card-foot">
      <span class="author">oleh <a href="${esc(p.author_url || '#')}" target="_blank" rel="noopener">${esc(p.author)}</a></span>
      <a class="repo" href="${esc(p.repo_url)}" target="_blank" rel="noopener">repo ↗</a>
    </div>
  </article>`;
}

/* ---------- daftar port ---------- */
let requestSeq = 0;

async function loadPorts() {
  const seq = ++requestSeq;
  $('#content').innerHTML = '<div class="state"><div class="spinner"></div><div class="big">Mengambil data…</div></div>';

  const params = new URLSearchParams({
    limit: state.limit, offset: state.offset, sort: state.sort
  });
  if (state.q) params.set('q', state.q);
  if (state.kategori) params.set('kategori', state.kategori);
  if (state.status) params.set('status', state.status);
  if (state.platform) params.set('platform', state.platform);

  let data;
  try {
    data = await fetchJson(`/api/ports?${params.toString()}`);
  } catch (err) {
    if (seq !== requestSeq) return;
    $('#resultInfo').textContent = 'gagal memuat';
    $('#content').innerHTML = `<div class="state">
      <div class="big">Gagal mengambil data dari backend.</div>
      <div class="mono" style="color:var(--bad)">${esc(err.message)}</div>
      <div style="margin-top:16px">Pastikan container <code>backend</code> dan <code>db</code> hidup.</div>
    </div>`;
    $('#pager').hidden = true;
    return;
  }

  if (seq !== requestSeq) return;   // respons usang

  $('#resultInfo').textContent = `${data.total.toLocaleString('id-ID')} port ditemukan`;
  const activeFilters = [
    state.q && `"${state.q}"`, state.kategori, state.status, state.platform
  ].filter(Boolean);
  $('#queryInfo').textContent = activeFilters.length ? `filter: ${activeFilters.join(' · ')}` : '';

  if (!data.items.length) {
    $('#content').innerHTML = `<div class="state">
      <div class="big">Tidak ada port yang cocok.</div>
      <div>Coba ubah kata kunci atau reset filter.</div>
    </div>`;
    $('#pager').hidden = true;
    return;
  }

  $('#content').innerHTML = `<div class="grid">${data.items.map(cardHtml).join('')}</div>`;

  const from = data.offset + 1;
  const to = Math.min(data.offset + data.limit, data.total);
  $('#pageInfo').textContent = `${from}–${to} / ${data.total}`;
  $('#prev').disabled = data.offset <= 0;
  $('#next').disabled = to >= data.total;
  $('#pager').hidden = data.total <= data.limit;
}

/* ---------- event ---------- */
let searchTimer;
$('#q').addEventListener('input', (e) => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    state.q = e.target.value.trim();
    state.offset = 0;
    loadPorts();
  }, 280);
});

$('#statusSel').addEventListener('change', (e) => {
  state.status = e.target.value; state.offset = 0; loadPorts();
});
$('#platformSel').addEventListener('change', (e) => {
  state.platform = e.target.value; state.offset = 0; loadPorts();
});
$('#sortSel').addEventListener('change', (e) => {
  state.sort = e.target.value; state.offset = 0; loadPorts();
});
$('#prev').addEventListener('click', () => {
  state.offset = Math.max(0, state.offset - state.limit);
  loadPorts();
  window.scrollTo({ top: 400, behavior: 'smooth' });
});
$('#next').addEventListener('click', () => {
  state.offset += state.limit;
  loadPorts();
  window.scrollTo({ top: 400, behavior: 'smooth' });
});
$('#catChips').addEventListener('click', (e) => {
  const btn = e.target.closest('.chip');
  if (!btn) return;
  document.querySelectorAll('#catChips .chip').forEach((b) => b.classList.remove('active'));
  btn.classList.add('active');
  state.kategori = btn.dataset.cat;
  state.offset = 0;
  loadPorts();
});

/* ---------- init ---------- */
(async function init() {
  await checkHealth();
  await Promise.all([loadStats(), loadFilters()]);
  await loadPorts();
  setInterval(checkHealth, 30000);
})();