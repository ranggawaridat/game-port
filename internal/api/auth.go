// Package api — autentikasi admin berbasis cookie sesi.
package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const (
	adminCookieName = "gp_admin"
	sessionTTL      = 12 * time.Hour
)

// hashPassword membuat hash SHA-256 dari password (dibandingkan konstan-waktu).
// Untuk produksi dengan banyak admin, ganti ke bcrypt/argon2.
func hashPassword(p string) string {
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:])
}

func randToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// cekPassword membandingkan password masukan dengan hash tersimpan.
func (s *Server) cekPassword(input string) bool {
	if s.cfg.AdminPasswordHash == "" {
		return false
	}
	got := hashPassword(input)
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.AdminPasswordHash)) == 1
}

// buatSesi menyimpan token sesi baru ke database.
func (s *Server) buatSesi(ctx context.Context) (string, time.Time, error) {
	token := randToken(32)
	if token == "" {
		return "", time.Time{}, errInternal("gagal membuat token")
	}
	exp := time.Now().Add(sessionTTL)
	_, err := s.store.Pool.Exec(ctx,
		`INSERT INTO admin_sessions (token, expires_at) VALUES ($1,$2)`, token, exp)
	if err != nil {
		return "", time.Time{}, err
	}
	// bersihkan sesi kedaluwarsa sekalian
	_, _ = s.store.Pool.Exec(ctx, `DELETE FROM admin_sessions WHERE expires_at < now()`)
	return token, exp, nil
}

// sesiValid memeriksa cookie terhadap tabel sesi.
func (s *Server) sesiValid(ctx context.Context, r *http.Request) bool {
	c, err := r.Cookie(adminCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	var ok bool
	err = s.store.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token=$1 AND expires_at > now())`,
		c.Value).Scan(&ok)
	if err != nil {
		return false
	}
	return ok
}

// hapusSesi menghapus token dari database.
func (s *Server) hapusSesi(ctx context.Context, token string) {
	_, _ = s.store.Pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token = $1`, token)
}

// wajibAdmin adalah middleware: 401 bila belum login.
// Sengaja mengembalikan 404 untuk request non-HTMX supaya endpoint admin
// tidak "mengumumkan" keberadaannya ke pengunjung biasa.
func (s *Server) wajibAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.sesiValid(r.Context(), r) {
			next(w, r)
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "text/html") ||
			r.Header.Get("HX-Request") == "true" {
			// halaman login admin: path acak supaya tidak terlihat publik
			http.Redirect(w, r, s.cfg.AdminPath, http.StatusSeeOther)
			return
		}
		// pura-pura tidak ada
		s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "tidak ditemukan"})
	}
}