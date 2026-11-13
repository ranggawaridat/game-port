package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ranggawaridat/game-port/internal/db"
)

// =============================================================================
// Safelink — gerbang iklan sebelum unduhan
//
// Alur:
//   GET  /dl/{slug}/{kind}        halaman gerbang; menerbitkan nonce
//   POST /dl/{slug}/{kind}/mint   menukar nonce -> token sekali pakai
//   GET  /api/ports/{slug}/download?t=TOKEN   menebus token -> 302 ke penyedia
//
// Selama SAFELINK_ENABLED=false seluruh route ini meneruskan ke perilaku lama,
// jadi fitur bisa ditinggal dalam keadaan mati tanpa memutus unduhan.
// =============================================================================

const (
	nonceUmur   = 2 * time.Hour // batas atas umur halaman gerbang
	settingIklan = "safelink_ad_html"
)

// cekToken memastikan permintaan unduhan membawa token sah bila safelink aktif.
// Token sekali pakai: pemakaian kedua langsung ditolak.
func (s *Server) cekToken(w http.ResponseWriter, r *http.Request, p *db.PortDetail, kind string) bool {
	if !s.cfg.SafeLinkEnabled {
		return true
	}
	t := r.URL.Query().Get("t")
	if t == "" {
		s.writeErr(w, http.StatusNotFound, "unduhan harus lewat halaman gerbang")
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	portID, kindToken, err := s.store.PakaiTokenDownload(ctx, hashToken(t))
	if err != nil || portID != p.ID || kindToken != kind {
		s.writeErr(w, http.StatusNotFound, "token tidak sah atau sudah dipakai")
		return false
	}
	return true
}

// ---- util acak ----

func acakHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

func (s *Server) hashIP(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.RemoteAddr
		if i := strings.LastIndex(ip, ":"); i > 0 {
			ip = ip[:i]
		}
	}
	return hashToken("ip:" + ip)[:16]
}

// ---- halaman gerbang ----

type safelinkData struct {
	Game     string
	Slug     string
	Kind     string
	KindText string
	Nonce    string
	Tunggu   int
	AdHTML   template.HTML
}

var safelinkTmpl = template.Must(template.New("safelink").Parse(`<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow, noarchive">
<title>Menyiapkan unduhan — {{.Game}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{background:#0a0a0b;color:#f4f4f5;font-family:Inter,system-ui,-apple-system,sans-serif;
  line-height:1.6;display:flex;flex-direction:column;min-height:100vh}
a{color:#a78bfa}
.bar{border-bottom:1px solid #26262b;padding:14px 18px;display:flex;align-items:center;gap:9px}
.bar b{font-size:15px;letter-spacing:-.01em}
.dot{width:9px;height:9px;border-radius:50%;background:#7c5cff}
.wrap{flex:1;max-width:640px;margin:0 auto;padding:28px 18px 40px;width:100%}
h1{font-size:clamp(19px,4.4vw,25px);letter-spacing:-.02em;line-height:1.25;margin-bottom:6px}
.sub{font-family:'JetBrains Mono',ui-monospace,monospace;font-size:11px;color:#71717a;
  text-transform:uppercase;letter-spacing:.09em;margin-bottom:22px}
.ad{min-height:250px;border:1px dashed #33333a;border-radius:12px;background:#111113;
  display:flex;align-items:center;justify-content:center;color:#52525b;
  font-family:ui-monospace,monospace;font-size:11px;margin-bottom:20px;overflow:hidden}
.count{text-align:center;margin-bottom:16px}
.count .n{font-family:ui-monospace,monospace;font-size:34px;font-weight:600;
  color:#a78bfa;line-height:1}
.count .l{font-size:11px;color:#71717a;margin-top:4px}
.btn{display:block;width:100%;padding:15px;border:0;border-radius:11px;
  background:#7c5cff;color:#fff;font-size:15px;font-weight:600;cursor:pointer;
  font-family:inherit;transition:.15s}
.btn:hover:not(:disabled){background:#6b4bf0}
.btn:disabled{background:#1f1f25;color:#52525b;cursor:default}
.msg{margin-top:14px;font-size:13px;text-align:center;color:#fbbf24;min-height:20px}
.msg.bad{color:#ef4444}
.foot{margin-top:26px;padding-top:16px;border-top:1px solid #26262b;
  font-family:ui-monospace,monospace;font-size:10.5px;color:#52525b}
</style>
</head>
<body>
<div class="bar"><span class="dot"></span><b>GamePort</b></div>

<!-- Umpan deteksi adblock. Selama tidak diblokir, ukurannya 1x1 piksel
     dan dipindah keluar layar sehingga tidak terlihat siapa pun. -->
<div id="adBait" class="adsbox ad-banner adsbygoogle"
     style="position:absolute;left:-9999px;top:-9999px;width:1px;height:1px"></div>

<div class="wrap">
  <h1>{{.Game}}</h1>
  <div class="sub">Menyiapkan unduhan · {{.KindText}}</div>

  <div class="ad" id="adSlot">{{if .AdHTML}}{{.AdHTML}}{{else}}slot iklan{{end}}</div>

  <div class="count">
    <div class="n" id="num">{{.Tunggu}}</div>
    <div class="l">detik lagi</div>
  </div>

  <button class="btn" id="go" disabled>Menyiapkan…</button>
  <div class="msg" id="msg"></div>

  <div class="foot">Halaman ini hanya penghubung. Berkas diunduh dari penyedia aslinya.</div>
</div>

<script>
(function(){
  var nonce   = {{.Nonce}};
  var tunggu  = {{.Tunggu}};
  var slug    = {{.Slug}};
  var kind    = {{.Kind}};
  var numEl   = document.getElementById('num');
  var btn     = document.getElementById('go');
  var msgEl   = document.getElementById('msg');
  var sisa    = tunggu;
  var selesai = false;

  // Adblock memblokir elemen dengan kelas/ID yang dikenali filternya.
  // Umpan disembunyikan lewat display:none atau ukuran 0 -> terdeteksi.
  function terblokir() {
    var b = document.getElementById('adBait');
    if (!b) return false;
    var cs = getComputedStyle(b);
    return b.offsetHeight === 0 || b.offsetWidth === 0 ||
           cs.display === 'none' || cs.visibility === 'hidden';
  }

  function kunciKarenaAdblock() {
    btn.disabled = true;
    btn.textContent = 'Adblock terdeteksi';
    msgEl.className = 'msg bad';
    msgEl.textContent = 'Matikan adblock / pemblokir iklan untuk halaman ini. Tombol akan aktif sendiri.';
  }

  function bukaTombol() {
    btn.disabled = false;
    btn.textContent = 'Lanjut ke unduhan';
    msgEl.className = 'msg';
    msgEl.textContent = '';
  }

  // Hitungan mundur. Tidak berhenti karena adblock; pengawas di bawah yang
  // menentukan tombol boleh dipakai atau tidak.
  var t = setInterval(function(){
    if (sisa > 0) {
      sisa--;
      numEl.textContent = sisa > 0 ? sisa : '0';
    }
    if (sisa <= 0) {
      clearInterval(t);
      selesai = true;
      if (!terblokir()) bukaTombol();
    }
  }, 1000);

  // Pengawas tetap: mengunci selama adblock aktif, dan membuka lagi kalau
  // pengguna mematikan pemblokirnya tanpa memuat ulang halaman.
  setInterval(function(){
    if (terblokir()) return kunciKarenaAdblock();
    if (selesai) bukaTombol();
  }, 1000);

  if (terblokir()) kunciKarenaAdblock();

  btn.addEventListener('click', function(){
    if (!selesai) return;
    if (terblokir()) { kunciKarenaAdblock(); return; }

    btn.disabled = true;
    btn.textContent = 'Membuka…';
    msgEl.className = 'msg';
    msgEl.textContent = '';

    fetch('/dl/' + encodeURIComponent(slug) + '/' + kind + '/mint', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({nonce: nonce, ad_ok: true})
    })
    .then(function(r){ return r.json().then(function(b){ return {ok:r.ok, body:b}; }); })
    .then(function(res){
      if (!res.ok) throw new Error(res.body && res.body.error ? res.body.error : 'gagal');
      window.location.href = res.body.url;
    })
    .catch(function(e){
      btn.disabled = false;
      btn.textContent = 'Coba lagi';
      msgEl.className = 'msg bad';
      msgEl.textContent = e.message;
    });
  });
})();
</script>
</body>
</html>`))

// handleSafeLinkPage menyajikan halaman gerbang dan menerbitkan nonce.
func (s *Server) handleSafeLinkPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	kind := r.PathValue("kind")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	p, err := s.store.GetPort(ctx, slug, false)
	if err != nil {
		s.writeErr(w, http.StatusNotFound, "port tidak ditemukan")
		return
	}
	if !urlTersedia(p, kind) {
		s.writeErr(w, http.StatusNotFound, "unduhan tidak tersedia")
		return
	}

	// Fitur mati: teruskan ke perilaku lama, tanpa halaman perantara.
	if !s.cfg.SafeLinkEnabled {
		s.tebusLangsung(w, r, p, kind)
		return
	}

	nonce, err := acakHex(24)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "gagal membuat nonce")
		return
	}
	if err := s.store.BuatNonceHalaman(ctx, nonce, p.ID, kind, s.hashIP(r)); err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Bersih-bersih sesekali, tanpa menahan render halaman.
	if n, err := rand.Int(rand.Reader, big.NewInt(20)); err == nil && n.Int64() == 0 {
		go func() {
			c, cc := context.WithTimeout(context.Background(), 5*time.Second)
			defer cc()
			_ = s.store.BersihkanSafelink(c)
		}()
	}

	iklan, _ := s.store.AmbilSetting(ctx, settingIklan)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")

	_ = safelinkTmpl.Execute(w, safelinkData{
		Game:     p.Game,
		Slug:     p.Slug,
		Kind:     kind,
		KindText: kindText(kind),
		Nonce:    nonce,
		Tunggu:   s.cfg.SafeLinkWait,
		AdHTML:   template.HTML(iklan), // sengaja: diisi admin, bukan pengunjung
	})
}

// handleSafeLinkMint menukar nonce menjadi token sekali pakai.
func (s *Server) handleSafeLinkMint(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.SafeLinkEnabled {
		s.writeErr(w, http.StatusNotFound, "tidak ditemukan")
		return
	}
	slug := r.PathValue("slug")
	kind := r.PathValue("kind")

	var in struct {
		Nonce string `json:"nonce"`
		AdOK  bool   `json:"ad_ok"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil {
		s.writeErr(w, http.StatusBadRequest, "permintaan tidak valid")
		return
	}
	if !in.AdOK {
		// Penanda ini datang dari halaman dan bisa dipalsukan; gunanya menahan
		// skrip sederhana, bukan penyerang serius. Nonce sengaja TIDAK dipakai
		// di sini supaya pengguna yang mematikan adblock bisa mencoba lagi
		// tanpa memuat ulang halaman.
		s.writeErr(w, http.StatusForbidden, "matikan adblock lalu coba lagi")
		return
	}
	if in.Nonce == "" {
		s.writeErr(w, http.StatusBadRequest, "nonce kosong")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tunggu := time.Duration(s.cfg.SafeLinkWait) * time.Second
	portID, kindNonce, err := s.store.PakaiNonceHalaman(ctx, in.Nonce, tunggu, nonceUmur)
	if errors.Is(err, db.ErrBelumWaktu) {
		s.writeErr(w, http.StatusTooManyRequests, "tunggu sebentar lagi")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusNotFound, "sesi tidak sah, muat ulang halaman")
		return
	}

	// Nonce harus milik port & jenis yang sama dengan URL.
	p, err := s.store.GetPort(ctx, slug, false)
	if err != nil || p.ID != portID || kindNonce != kind {
		s.writeErr(w, http.StatusNotFound, "sesi tidak cocok dengan unduhan ini")
		return
	}

	token, err := acakHex(32)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "gagal membuat token")
		return
	}
	ttl := time.Duration(s.cfg.SafeLinkTTL) * time.Second
	if err := s.store.TerbitkanTokenDownload(ctx, hashToken(token), portID, kind, s.hashIP(r), ttl); err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{
		"url": s.tebusURL(p.Slug, kind, token),
	})
}

// ---- pembantu ----

func kindText(kind string) string {
	if kind == "iso" {
		return "ISO / data game"
	}
	return "port Android"
}

func urlTersedia(p *db.PortDetail, kind string) bool {
	switch kind {
	case "port":
		return p.DownloadURL != nil && *p.DownloadURL != ""
	case "iso":
		return p.IsoURL != nil && *p.IsoURL != ""
	}
	return false
}

func (s *Server) tebusURL(slug, kind, token string) string {
	rute := "/api/v1/ports/" + slug + "/download"
	if kind == "iso" {
		rute = "/api/v1/ports/" + slug + "/iso"
	}
	return rute + "?t=" + token
}

// tebusLangsung dipakai saat safelink dimatikan.
func (s *Server) tebusLangsung(w http.ResponseWriter, r *http.Request, p *db.PortDetail, kind string) {
	target := p.DownloadURL
	if kind == "iso" {
		target = p.IsoURL
	}
	if target == nil || *target == "" {
		s.writeErr(w, http.StatusNotFound, "unduhan tidak tersedia")
		return
	}
	http.Redirect(w, r, *target, http.StatusFound)
}

