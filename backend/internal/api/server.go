// Package api menyediakan router HTTP: API publik, admin, dan static.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ranggawaridat/game-port/internal/config"
	"github.com/ranggawaridat/game-port/internal/db"
)

// Server membungkus dependensi handler.
type Server struct {
	cfg   *config.Config
	store *db.Store
	log   *slog.Logger
}

// New membuat Server.
func New(cfg *config.Config, store *db.Store, log *slog.Logger) *Server {
	return &Server{cfg: cfg, store: store, log: log}
}

type internalErr struct{ msg string }

func (e internalErr) Error() string { return e.msg }
func errInternal(m string) error    { return internalErr{m} }

// Routes memasang semua rute.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// ---------- Publik (ber-version) ----------
	//
	// Pola ditulis TANPA awalan, lalu dipasang di dua awalan:
	//   /v1/...  versi API untuk frontend terpisah (api.gameport.web.id)
	//   /api/... alias lama yang masih dipakai frontend monolitik
	//
	// Alias lama hanya dijaga selama masa migrasi frontend; setelah
	// frontend terpisah benar-benar live, alias ini boleh dibuang.
	publik := []struct {
		pola string
		h    http.HandlerFunc
	}{
		{"/health", s.handleHealth},
		{"/statistik", s.handleStatistik},
		{"/kategori", s.handleKategori},
		{"/status", s.handleStatus},
		{"/platform", s.handlePlatform},
		{"/genre", s.handleGenre},
		{"/ports", s.handlePorts},
		{"/ports/{slug}", s.handlePortDetail},
		{"/ports/{slug}/download", s.handleDownload},
		{"/ports/{slug}/iso", s.handleIsoDownload},
	}
	for _, pre := range []string{"/api/v1", "/api"} {
		for _, r := range publik {
			mux.HandleFunc("GET "+pre+r.pola, r.h)
		}
	}

	// Safelink: gerbang iklan sebelum unduhan (halaman publik, tanpa versi)
	mux.HandleFunc("GET /dl/{slug}/{kind}", s.handleSafeLinkPage)
	mux.HandleFunc("POST /dl/{slug}/{kind}/mint", s.handleSafeLinkMint)

	// ---------- Admin (path rahasia, tidak ditautkan dari publik) ----------
	ap := s.cfg.AdminPath
	mux.HandleFunc("GET "+ap, s.handleAdminPage)
	mux.HandleFunc("POST "+ap+"/login", s.handleAdminLogin)
	mux.HandleFunc("POST "+ap+"/logout", s.handleAdminLogout)

	mux.HandleFunc("GET "+ap+"/api/stat", s.wajibAdmin(s.handleAdminStat))
	mux.HandleFunc("GET "+ap+"/api/list", s.wajibAdmin(s.handleAdminList))
	mux.HandleFunc("GET "+ap+"/api/port/{id}", s.wajibAdmin(s.handleAdminGet))
	mux.HandleFunc("POST "+ap+"/api/port", s.wajibAdmin(s.handleAdminCreate))
	mux.HandleFunc("PUT "+ap+"/api/port/{id}", s.wajibAdmin(s.handleAdminUpdate))
	mux.HandleFunc("DELETE "+ap+"/api/port/{id}", s.wajibAdmin(s.handleAdminDelete))
	mux.HandleFunc("POST "+ap+"/api/upload", s.wajibAdmin(s.handleAdminUpload))
	mux.HandleFunc("GET "+ap+"/api/settings", s.wajibAdmin(s.handleAdminGetSettings))
	mux.HandleFunc("POST "+ap+"/api/settings", s.wajibAdmin(s.handleAdminSaveSettings))

	// ---------- Static ----------
	mux.HandleFunc("GET /", s.handleStatic)
	mux.HandleFunc("GET /uploads/{file}", s.handleUpload)

	return s.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// CORS untuk endpoint publik yang diakses frontend terpisah.
		// /api/v1 adalah prefix baru; /api/... dijaga sebagai alias migrasi.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if origin := s.allowedOrigin(r.Header.Get("Origin")); origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Jangan cache halaman admin
		if strings.HasPrefix(r.URL.Path, s.cfg.AdminPath) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}

		next.ServeHTTP(w, r)
		s.log.Info("req", "m", r.Method, "p", r.URL.Path, "d", time.Since(start).Round(time.Microsecond))
	})
}

func (s *Server) allowedOrigin(origin string) string {
	for _, o := range s.cfg.CORSOrigins {
		if o == "*" {
			return "*"
		}
		if strings.EqualFold(o, origin) {
			return origin
		}
	}
	return ""
}

func (s *Server) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Error("encode", "err", err)
	}
}

func (s *Server) writeErr(w http.ResponseWriter, code int, msg string) {
	s.writeJSON(w, code, map[string]string{"error": msg})
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '|'
	}) {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------- Handler publik ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	info, err := s.store.Health(ctx)
	if err != nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleStatistik(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	v, err := s.store.Statistik(ctx)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleKategori(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	v, err := s.store.KategoriList(ctx)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	v, err := s.store.StatusList(ctx)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, v)
}

func (s *Server) handlePlatform(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	v, err := s.store.PlatformList(ctx)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleGenre(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	v, err := s.store.GenreList(ctx)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, v)
}

func (s *Server) parseFilter(r *http.Request, admin bool) (db.PortFilter, error) {
	q := r.URL.Query()
	f := db.PortFilter{
		Q:         strings.TrimSpace(q.Get("q")),
		Kategori:  splitCSV(q.Get("kategori")),
		Status:    splitCSV(q.Get("status")),
		Platform:  splitCSV(q.Get("platform")),
		Genre:     splitCSV(q.Get("genre")),
		Sort:      q.Get("sort"),
		Limit:     24,
		Offset:    0,
		AdminOnly: admin,
	}
	if v := q.Get("own"); v == "0" || v == "1" {
		b := v == "1"
		f.Own = &b
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, errors.New("limit harus angka")
		}
		if n < 1 {
			n = 1
		}
		if n > 200 {
			n = 200
		}
		f.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, errors.New("offset harus angka")
		}
		if n < 0 {
			n = 0
		}
		f.Offset = n
	}
	return f, nil
}

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	f, err := s.parseFilter(r, false)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// catat pencarian (tanpa memblokir respons)
	if f.Q != "" {
		q := f.Q
		go func() {
			c, cc := context.WithTimeout(context.Background(), 3*time.Second)
			defer cc()
			_ = s.store.CatatEvent(c, "search", nil, &q)
		}()
	}

	res, err := s.store.ListPorts(ctx, f)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handlePortDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	p, err := s.store.GetPort(ctx, slug, false)
	if errors.Is(err, db.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "port tidak ditemukan")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// catat view
	go func(id int64) {
		c, cc := context.WithTimeout(context.Background(), 3*time.Second)
		defer cc()
		s.store.TambahView(c, id)
		_ = s.store.CatatEvent(c, "view", &id, nil)
	}(p.ID)

	s.writeJSON(w, http.StatusOK, p)
}

// handleDownload menaikkan penghitung lalu mengalihkan ke URL unduhan.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	p, err := s.store.GetPort(ctx, slug, false)
	if err != nil || p.DownloadURL == nil || *p.DownloadURL == "" {
		s.writeErr(w, http.StatusNotFound, "unduhan tidak tersedia")
		return
	}
	if !s.cekToken(w, r, p, "port") {
		return
	}
	id := p.ID
	s.store.TambahDownload(ctx, id)
	_ = s.store.CatatEvent(ctx, "download", &id, nil)

	http.Redirect(w, r, *p.DownloadURL, http.StatusFound)
}

// handleIsoDownload mengarahkan ke link ISO/game (opsional) dan mencatat event.
func (s *Server) handleIsoDownload(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	p, err := s.store.GetPort(ctx, slug, false)
	if err != nil || p.IsoURL == nil || *p.IsoURL == "" {
		s.writeErr(w, http.StatusNotFound, "link ISO tidak tersedia")
		return
	}
	if !s.cekToken(w, r, p, "iso") {
		return
	}
	id := p.ID
	s.store.TambahDownload(ctx, id)
	_ = s.store.CatatEvent(ctx, "iso", &id, nil)

	http.Redirect(w, r, *p.IsoURL, http.StatusFound)
}

// handleStatic menyajikan frontend publik.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	name = strings.TrimPrefix(name, "static/")
	if name == "" {
		name = "index.html"
	}
	// Path admin TIDAK dilayani sebagai file statis
	if strings.HasPrefix(r.URL.Path, s.cfg.AdminPath) {
		s.writeErr(w, http.StatusNotFound, "tidak ditemukan")
		return
	}
	clean := filepath.Clean("/" + name)
	if strings.Contains(clean, "..") {
		s.writeErr(w, http.StatusBadRequest, "path tidak valid")
		return
	}
	full := filepath.Join(s.cfg.StaticDir, clean)
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	if !strings.Contains(name, ".") {
		idx := filepath.Join(s.cfg.StaticDir, "index.html")
		if _, e := os.Stat(idx); e == nil {
			http.ServeFile(w, r, idx)
			return
		}
	}
	s.writeErr(w, http.StatusNotFound, "tidak ditemukan")
}

// handleUpload menyajikan berkas gambar hasil unggahan admin.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("file"))
	if name == "" || strings.Contains(name, "..") {
		s.writeErr(w, http.StatusBadRequest, "nama berkas tidak valid")
		return
	}
	full := filepath.Join(s.cfg.UploadDir, name)
	if _, err := os.Stat(full); err != nil {
		s.writeErr(w, http.StatusNotFound, "gambar tidak ditemukan")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, full)
}

var _ = fmt.Sprintf