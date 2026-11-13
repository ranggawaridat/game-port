/* GamePort — panel admin. Path API mengikuti ADMIN_PATH dari server. */
'use strict';

const $ = (s) => document.querySelector(s);
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const fmt = (n) => Number(n || 0).toLocaleString('id-ID');

// Path admin diambil dari URL saat ini (mis. /panel-x7k2)
const BASE = location.pathname.replace(/\/+$/, '');

let refs = { kategori: [], status: [], platform: [], genre: [] };
let listState = { q: '', limit: 20, offset: 0, sort: 'terbaru' };

async function api(path, opts = {}) {
  const res = await fetch(BASE + '/api' + path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  });
  if (res.status === 401) { tampilLogin(); throw new Error('belum login'); }
  let data = null;
  try { data = await res.json(); } catch (_) {}
  if (!res.ok) throw new Error((data && data.error) || `HTTP ${res.status}`);
  return data;
}

function toast(msg, bad = false) {
  const t = $('#toast');
  t.textContent = msg;
  t.className = 'toast' + (bad ? ' bad' : '');
  t.hidden = false;
  clearTimeout(t._h);
  t._h = setTimeout(() => { t.hidden = true; }, 3200);
}

/* ---------------- login ---------------- */
function tampilLogin() {
  $('#appView').hidden = true;
  $('#loginView').hidden = false;
  setTimeout(() => $('#pw').focus(), 60);
}
function tampilApp() {
  $('#loginView').hidden = true;
  $('#appView').hidden = false;
}

$('#loginForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const btn = $('#loginBtn'), err = $('#loginErr');
  btn.disabled = true; err.hidden = true;
  try {
    const res = await fetch(BASE + '/login', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: $('#pw').value }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || 'gagal masuk');
    $('#pw').value = '';
    tampilApp();
    await muatAwal();
  } catch (ex) {
    err.textContent = ex.message;
    err.hidden = false;
  } finally {
    btn.disabled = false;
  }
});

$('#logoutBtn').addEventListener('click', async () => {
  try { await fetch(BASE + '/logout', { method: 'POST', credentials: 'same-origin' }); } catch (_) {}
  tampilLogin();
});

/* ---------------- tab ---------------- */
document.querySelectorAll('.nav-item').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.nav-item').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    const tab = btn.dataset.tab;
    document.querySelectorAll('.tab').forEach(t => { t.hidden = true; });
    $('#tab-' + tab).hidden = false;
    if (tab === 'stat') muatStat();
    if (tab === 'list') muatList();
    if (tab === 'safelink') muatSafelink();
  });
});

/* ---------------- safelink ---------------- */
async function muatSafelink() {
  const msg = $('#slMsg');
  try {
    const d = await api('/settings');
    $('#slAd').value = d.safelink_ad_html || '';
    $('#slStatus').textContent = d.safelink_enabled
      ? `aktif — tunggu ${d.safelink_wait}s, token ${d.safelink_ttl}s`
      : 'nonaktif — unduhan langsung ke penyedia';
    msg.textContent = '';
  } catch (e) { msg.textContent = e.message; }
}

$('#slSave').addEventListener('click', async () => {
  const btn = $('#slSave'), msg = $('#slMsg');
  btn.disabled = true; msg.textContent = 'menyimpan…';
  try {
    await api('/settings', {
      method: 'POST',
      body: JSON.stringify({ safelink_ad_html: $('#slAd').value }),
    });
    msg.textContent = 'tersimpan';
    toast('Script iklan disimpan');
  } catch (e) {
    msg.textContent = e.message;
    toast(e.message, true);
  } finally {
    btn.disabled = false;
  }
});

/* ---------------- statistik ---------------- */
async function muatStat() {
  try {
    const d = await api('/stat');
    const r = d.ringkasan;
    const cards = [
      [r.total_port, 'Total Port', ''],
      [r.terbit, 'Terbit', 'ok'],
      [r.draf, 'Draf', ''],
      [r.total_view, 'Total View', 'accent'],
      [r.total_download, 'Total Unduh', 'accent'],
      [r.view_hari_ini, 'View Hari Ini', ''],
      [r.download_hari_ini, 'Unduh Hari Ini', ''],
      [r.view_7hari, 'View 7 Hari', ''],
    ];
    $('#statCards').innerHTML = cards.map(([v, l, c]) =>
      `<div class="scard ${c}"><div class="v">${fmt(v)}</div><div class="l">${l}</div></div>`
    ).join('');

    // grafik: bar + sumbu terpisah supaya label tidak menimpa legenda
    const g = d.grafik || [];
    const max = Math.max(1, ...g.map(x => Math.max(x.view, x.download)));
    const totalView = g.reduce((a, x) => a + x.view, 0);
    const totalDl = g.reduce((a, x) => a + x.download, 0);

    if (!g.length) {
      $('#chart').innerHTML = '<div class="chart-empty">Belum ada data</div>';
      $('#chart').classList.remove('chart');
    } else {
      // #chart SENDIRI yang jadi flex container (jangan bikin .chart bersarang)
      $('#chart').classList.add('chart');
      $('#chart').innerHTML = g.map(x => {
        const hv = x.view === 0 ? 0 : Math.max(3, Math.round((x.view / max) * 100));
        const hd = x.download === 0 ? 0 : Math.max(3, Math.round((x.download / max) * 100));
        return `<div class="bar" title="${x.tanggal}: ${x.view} view, ${x.download} unduh">
          <div class="b" style="height:${hv}%"></div>
          <div class="t" style="height:${hd}%"></div>
        </div>`;
      }).join('');
      // sumbu tanggal sebagai elemen terpisah setelah #chart
      let axis = document.querySelector('.axis');
      if (!axis) {
        axis = document.createElement('div');
        axis.className = 'axis';
        $('#chart').after(axis);
      }
      axis.innerHTML = g.map(x => `<span>${x.tanggal.slice(8,10)}/${x.tanggal.slice(5,7)}</span>`).join('');
    }

    // legenda: letakkan SETELAH #chart, jangan menimpa label
    let legend = document.querySelector('.legend');
    if (!legend) {
      legend = document.createElement('div');
      legend.className = 'legend';
      $('#chart').parentElement.appendChild(legend);
    }
    legend.innerHTML = `<span><i style="background:#7c5cff"></i>View (${fmt(totalView)})</span>` +
                       `<span><i style="background:rgba(34,197,94,.75)"></i>Unduh (${fmt(totalDl)})</span>`;

    $('#populer').innerHTML = (d.populer || []).length
      ? d.populer.map(p => `<div class="lrow"><span class="nm">${esc(p.game)}</span>
          <span class="vl">${fmt(p.view_count)} view</span></div>`).join('')
      : '<div class="empty">Belum ada data</div>';

    $('#pencarian').innerHTML = (d.pencarian || []).length
      ? d.pencarian.map(p => `<div class="lrow"><span class="nm">${esc(p.query)}</span>
          <span class="vl">${fmt(p.jumlah)}×</span></div>`).join('')
      : '<div class="empty">Belum ada pencarian</div>';
  } catch (e) { toast(e.message, true); }
}
$('#refreshStat').addEventListener('click', muatStat);

/* ---------------- daftar ---------------- */
async function muatList() {
  const p = new URLSearchParams({ limit: listState.limit, offset: listState.offset, sort: listState.sort });
  if (listState.q) p.set('q', listState.q);
  try {
    const d = await api('/list?' + p.toString());
    if (!d.items.length) {
      $('#listTable').innerHTML = '<div class="empty" style="padding:40px">Belum ada port. Tambahkan lewat tab "Tambah Port".</div>';
      $('#listPager').hidden = true;
      return;
    }
    $('#listTable').innerHTML = `<table><thead><tr>
      <th></th><th>Game</th><th>Versi</th><th>Ukuran</th><th>Genre</th><th>Status</th>
      <th>View</th><th>Unduh</th><th>Aksi</th></tr></thead><tbody>
      ${d.items.map(p => `<tr>
        <td>${p.image_url
          ? `<img class="thumb-sm" src="${esc(p.image_url)}" alt="" loading="lazy">`
          : `<div class="thumb-sm ph">—</div>`}</td>
        <td><strong>${esc(p.game)}</strong>${p.published ? '' : ' <span class="pill draft">draf</span>'}</td>
        <td class="vl">${esc(p.version || '—')}</td>
        <td class="vl">${esc(p.file_size || '—')}</td>
        <td>${esc(p.genre_name || '—')}</td>
        <td>${esc(p.status_name)}</td>
        <td class="vl">${fmt(p.view_count)}</td>
        <td class="vl">${fmt(p.download_count)}</td>
        <td><div class="acts">
          <button data-edit="${p.id}">Edit</button>
          <button class="del" data-del="${p.id}" data-nama="${esc(p.game)}">Hapus</button>
        </div></td></tr>`).join('')}
      </tbody></table>`;

    const from = d.offset + 1, to = Math.min(d.offset + d.limit, d.total);
    $('#listInfo').textContent = `${from}–${to} / ${d.total}`;
    $('#listPrev').disabled = d.offset <= 0;
    $('#listNext').disabled = to >= d.total;
    $('#listPager').hidden = d.total <= d.limit;

    $('#listTable').querySelectorAll('[data-edit]').forEach(b =>
      b.addEventListener('click', () => editPort(Number(b.dataset.edit))));
    $('#listTable').querySelectorAll('[data-del]').forEach(b =>
      b.addEventListener('click', () => hapusPort(Number(b.dataset.del), b.dataset.nama)));
  } catch (e) { toast(e.message, true); }
}
$('#refreshList').addEventListener('click', muatList);
$('#listPrev').addEventListener('click', () => { listState.offset = Math.max(0, listState.offset - listState.limit); muatList(); });
$('#listNext').addEventListener('click', () => { listState.offset += listState.limit; muatList(); });
let qt;
$('#listQ').addEventListener('input', e => {
  clearTimeout(qt);
  qt = setTimeout(() => { listState.q = e.target.value.trim(); listState.offset = 0; muatList(); }, 280);
});

async function hapusPort(id, nama) {
  if (!confirm(`Hapus "${nama}"? Tindakan ini tidak bisa dibatalkan.`)) return;
  try { await api('/port/' + id, { method: 'DELETE' }); toast('Port dihapus'); muatList(); }
  catch (e) { toast(e.message, true); }
}

/* ---------------- form ---------------- */
function isiPilihan() {
  $('#f_category').innerHTML = '<option value="">— pilih kategori —</option>' +
    refs.kategori.map(k => `<option value="${esc(k.slug)}">${esc(k.name)}</option>`).join('');
  $('#f_status').innerHTML = '<option value="">— pilih status —</option>' +
    refs.status.map(s => `<option value="${esc(s.slug)}">${esc(s.name)}</option>`).join('');
  $('#f_genre').innerHTML = '<option value="">— pilih genre —</option>' +
    refs.genre.map(g => `<option value="${esc(g.slug)}">${esc(g.name)}</option>`).join('');
  $('#platformBox').innerHTML = refs.platform.map(p =>
    `<label><input type="checkbox" value="${esc(p.slug)}"> ${esc(p.name)}</label>`).join('');
}

function kosongkanForm() {
  $('#portForm').reset();
  $('#f_id').value = '';
  $('#formTitle').textContent = 'Tambah Port';
  $('#preview').hidden = true;
  $('#uploadMsg').textContent = '';
  $('#saveBtn').textContent = 'Simpan';
  document.querySelectorAll('#platformBox input').forEach(c => { c.checked = false; });
  $('#f_published').checked = true;
}

async function editPort(id) {
  try {
    const p = await api('/port/' + id);
    $('#f_id').value = p.id;
    $('#f_game').value = p.game || '';
    $('#f_version').value = p.version || '';
    $('#f_size').value = p.file_size || '';
    $('#f_genre').value = p.genre_slug || '';
    $('#f_category').value = p.category_slug || '';
    $('#f_status').value = p.status_slug || '';
    $('#f_author').value = p.author || '';
    $('#f_download').value = p.download_url || '';
    $('#f_iso').value = p.iso_url || '';
    $('#f_source').value = p.source_url || '';
    $('#f_notes').value = p.install_notes || '';
    $('#f_desc').value = p.description || '';
    $('#f_image').value = p.image_url || '';
    $('#f_published').checked = !!p.published;
    $('#f_owndata').checked = !!p.needs_own_data;
    document.querySelectorAll('#platformBox input').forEach(c => {
      c.checked = (p.platforms || []).includes(c.value);
    });
    if (p.image_url) { $('#previewImg').src = p.image_url; $('#preview').hidden = false; }
    else $('#preview').hidden = true;

    $('#formTitle').textContent = 'Edit Port #' + p.id;
    $('#saveBtn').textContent = 'Simpan Perubahan';
    document.querySelector('.nav-item[data-tab="form"]').click();
    window.scrollTo({ top: 0, behavior: 'smooth' });
  } catch (e) { toast(e.message, true); }
}

$('#portForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const btn = $('#saveBtn'), msg = $('#formMsg');
  btn.disabled = true; msg.textContent = 'menyimpan…';

  const platforms = Array.from(document.querySelectorAll('#platformBox input:checked')).map(c => c.value);
  const id = $('#f_id').value;
  const body = {
    game: $('#f_game').value.trim(),
    version: $('#f_version').value.trim(),
    file_size: $('#f_size').value.trim(),
    genre_slug: $('#f_genre').value || null,
    description: $('#f_desc').value.trim(),
    author: $('#f_author').value.trim(),
    category_slug: $('#f_category').value,
    status_slug: $('#f_status').value,
    image_url: $('#f_image').value.trim() || null,
    download_url: $('#f_download').value.trim() || null,
    iso_url: $('#f_iso').value.trim() || null,
    source_url: $('#f_source').value.trim() || null,
    install_notes: $('#f_notes').value.trim() || null,
    published: $('#f_published').checked,
    needs_own_data: $('#f_owndata').checked,
    platforms,
  };

  try {
    if (id) {
      await api('/port/' + id, { method: 'PUT', body: JSON.stringify(body) });
      toast('Perubahan disimpan');
    } else {
      await api('/port', { method: 'POST', body: JSON.stringify(body) });
      toast('Port baru ditambahkan');
    }
    kosongkanForm();
    muatStat();
  } catch (ex) {
    msg.textContent = '';
    toast(ex.message, true);
  } finally {
    btn.disabled = false;
    msg.textContent = '';
  }
});

$('#resetForm').addEventListener('click', kosongkanForm);
$('#cancelEdit').addEventListener('click', kosongkanForm);

/* ---------------- upload gambar ---------------- */
$('#pickFile').addEventListener('click', () => $('#fileInput').click());

$('#fileInput').addEventListener('change', async (e) => {
  const f = e.target.files && e.target.files[0];
  if (!f) return;
  if (f.size > 5 * 1024 * 1024) { toast('Gambar maksimal 5 MB', true); return; }

  const msg = $('#uploadMsg');
  msg.textContent = 'mengunggah…';
  const fd = new FormData();
  fd.append('file', f);
  try {
    const res = await fetch(BASE + '/api/upload', {
      method: 'POST', credentials: 'same-origin', body: fd,
    });
    const data = await res.json().catch(() => ({}));
    if (res.status === 401) { tampilLogin(); return; }
    if (!res.ok) throw new Error(data.error || 'gagal mengunggah');
    $('#f_image').value = data.url;
    $('#previewImg').src = data.url;
    $('#preview').hidden = false;
    msg.textContent = 'berhasil diunggah';
    toast('Gambar terunggah');
  } catch (ex) {
    msg.textContent = '';
    toast(ex.message, true);
  } finally {
    e.target.value = '';
  }
});

$('#f_image').addEventListener('change', (e) => {
  const v = e.target.value.trim();
  if (v) { $('#previewImg').src = v; $('#preview').hidden = false; }
  else $('#preview').hidden = true;
});

/* ---------------- init ---------------- */
async function muatAwal() {
  try {
    const [k, s, p, g] = await Promise.all([
      api('/kategori').catch(() => fetch('/api/kategori').then(r => r.json())),
      api('/status').catch(() => fetch('/api/status').then(r => r.json())),
      api('/platform').catch(() => fetch('/api/platform').then(r => r.json())),
      api('/genre').catch(() => fetch('/api/genre').then(r => r.json())),
    ]);
    refs = { kategori: k, status: s, platform: p, genre: g };
    isiPilihan();
  } catch (e) { toast(e.message, true); }
  muatStat();
}

(async function init() {
  // Cek apakah sesi masih berlaku
  try {
    await api('/stat');
    tampilApp();
    await muatAwal();
  } catch (_) {
    tampilLogin();
  }
})();