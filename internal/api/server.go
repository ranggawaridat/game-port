// Package api menyediakan router HTTP dan handler JSON.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ranggawaridat/game-port/internal/config"
	"github.com/ranggawaridat/game-port/internal/db"

	"github.com/jackc/pgx/v5"
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

// Routes memasang semua rute pada ServeMux.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/statistik", s.handleStatistik)
	mux.HandleFunc("GET /api/kategori", s.handleKategori)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/platform", s.handlePlatform)
	mux.HandleFunc("GET /api/engine", s.handleEngine)
	mux.HandleFunc("GET /api/ports", s.handlePorts)
	mux.HandleFunc("GET /api/ports/{slug}", s.handlePortDetail)

	mux.HandleFunc("GET /", s.handleStatic)

	return s.middleware(mux)
}

// ---------- middleware ----------

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// CORS
		if origin := s.allowedOrigin(r.Header.Get("Origin")); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path,
			"durasi", time.Since(start).String())
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

// ---------- helper JSON ----------

func (s *Server) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Error("encode json", "err", err)
	}
}

func (s *Server) writeErr(w http.ResponseWriter, code int, msg string) {
	s.writeJSON(w, code, map[string]string{"error": msg})
}

// ---------- handler ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	info, err := s.store.Health(ctx)
	if err != nil {
		s.log.Error("health", "err", err)
		s.writeJSON(w, http.StatusServiceUnavailable,
			map[string]any{"ok": false, "error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleStatistik(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	st, err := s.store.Statistik(ctx)
	if err != nil {
		s.log.Error("statistik", "err", err)
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleKategori(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	v, err := s.store.KategoriList(ctx)
	if err != nil {
		s.log.Error("kategori", "err", err)
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
		s.log.Error("status", "err", err)
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
		s.log.Error("platform", "err", err)
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleEngine(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	v, err := s.store.EngineList(ctx)
	if err != nil {
		s.log.Error("engine", "err", err)
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, v)
}

// splitCSV memecah "a,b c" menjadi []string tanpa entri kosong.
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

func (s *Server) parseFilter(r *http.Request) (db.PortFilter, error) {
	q := r.URL.Query()

	f := db.PortFilter{
		Q:        strings.TrimSpace(q.Get("q")),
		Kategori: splitCSV(q.Get("kategori")),
		Status:   splitCSV(q.Get("status")),
		Platform: splitCSV(q.Get("platform")),
		Sort:     q.Get("sort"),
		Limit:    24,
		Offset:   0,
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
	f, err := s.parseFilter(r)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	res, err := s.store.ListPorts(ctx, f)
	if err != nil {
		s.log.Error("daftar ports", "err", err)
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handlePortDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		s.writeErr(w, http.StatusBadRequest, "slug kosong")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	p, err := s.store.GetPort(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		s.writeErr(w, http.StatusNotFound, "port tidak ditemukan: "+slug)
		return
	}
	if err != nil {
		s.log.Error("detail port", "slug", slug, "err", err)
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, p)
}

// handleStatic menyajikan file dari STATIC_DIR.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	// URL /static/x.css -> berkas STATIC_DIR/x.css
	name = strings.TrimPrefix(name, "static/")
	if name == "" {
		name = "index.html"
	}
	// Cegah path traversal
	clean := filepath.Clean("/" + name)
	if strings.Contains(clean, "..") {
		s.writeErr(w, http.StatusBadRequest, "path tidak valid")
		return
	}
	full := filepath.Join(s.cfg.StaticDir, clean)

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		// Fallback SPA: rute tanpa ekstensi -> index.html
		if !strings.Contains(name, ".") {
			idx := filepath.Join(s.cfg.StaticDir, "index.html")
			if _, e := os.Stat(idx); e == nil {
				http.ServeFile(w, r, idx)
				return
			}
		}
		s.writeErr(w, http.StatusNotFound, "file tidak ditemukan: "+name)
		return
	}
	http.ServeFile(w, r, full)
}