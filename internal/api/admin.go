package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ranggawaridat/game-port/internal/db"
)

// handleAdminPage menyajikan panel admin (HTML tunggal).
func (s *Server) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	// Jika belum login, tetap sajikan shell; JS akan memanggil /api/stat
	// dan jika 401 menampilkan form login.
	fp := filepath.Join(s.cfg.StaticDir, "admin.html")
	if _, err := os.Stat(fp); err != nil {
		s.writeErr(w, http.StatusNotFound, "admin.html belum ada")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, fp)
}

// handleAdminLogin memeriksa password dan memasang cookie sesi.
func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		s.writeErr(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	if !s.cekPassword(body.Password) {
		// jeda kecil untuk memperlambat brute force
		time.Sleep(600 * time.Millisecond)
		s.writeErr(w, http.StatusUnauthorized, "password salah")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	token, exp, err := s.buatSesi(ctx)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
		// Secure: true  -> aktifkan bila sudah HTTPS di Caddy
		Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAdminLogout menghapus sesi.
func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(adminCookieName); err == nil {
		s.destroySession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: adminCookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) destroySession(ctx context.Context, token string) {
	s.hapusSesi(ctx, token)
}

// ---------- Statistik admin ----------

func (s *Server) handleAdminStat(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	st, err := s.store.StatAdmin(ctx)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	grafik, err := s.store.GrafikHarian(ctx, 14)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	populer, err := s.store.PortPopuler(ctx, 8)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	cari, err := s.store.PencarianTeratas(ctx, 10)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"ringkasan": st,
		"grafik":    grafik,
		"populer":   populer,
		"pencarian": cari,
	})
}

// ---------- CRUD ----------

func (s *Server) handleAdminList(w http.ResponseWriter, r *http.Request) {
	f, err := s.parseFilter(r, true)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if f.Sort == "" {
		f.Sort = "terbaru"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	res, err := s.store.ListPorts(ctx, f)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAdminGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, "id tidak valid")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	p, err := s.store.GetPortByID(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "port tidak ditemukan")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, p)
}

func decodePortInput(r *http.Request) (db.PortInput, error) {
	var in db.PortInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return in, fmt.Errorf("JSON tidak valid: %w", err)
	}
	in.Game = strings.TrimSpace(in.Game)
	if in.Game == "" {
		return in, errors.New("nama game wajib diisi")
	}
	if strings.TrimSpace(in.CategorySlug) == "" {
		return in, errors.New("kategori wajib dipilih")
	}
	if strings.TrimSpace(in.StatusSlug) == "" {
		return in, errors.New("status wajib dipilih")
	}
	if in.ImageURL != nil {
		v := strings.TrimSpace(*in.ImageURL)
		if v == "" {
			in.ImageURL = nil
		} else {
			in.ImageURL = &v
		}
	}
	if in.DownloadURL != nil {
		v := strings.TrimSpace(*in.DownloadURL)
		if v == "" {
			in.DownloadURL = nil
		} else {
			in.DownloadURL = &v
		}
	}
	if in.IsoURL != nil {
		v := strings.TrimSpace(*in.IsoURL)
		if v == "" {
			in.IsoURL = nil
		} else {
			in.IsoURL = &v
		}
	}
	if in.GenreSlug != nil {
		v := strings.TrimSpace(*in.GenreSlug)
		if v == "" {
			in.GenreSlug = nil
		} else {
			in.GenreSlug = &v
		}
	}
	return in, nil
}

func (s *Server) handleAdminCreate(w http.ResponseWriter, r *http.Request) {
	in, err := decodePortInput(r)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	p, err := s.store.CreatePort(ctx, in)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleAdminUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, "id tidak valid")
		return
	}
	in, err := decodePortInput(r)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	p, err := s.store.UpdatePort(ctx, id, in)
	if errors.Is(err, db.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "port tidak ditemukan")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleAdminDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, "id tidak valid")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.store.DeletePort(ctx, id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			s.writeErr(w, http.StatusNotFound, "port tidak ditemukan")
			return
		}
		s.writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- Upload gambar ----------

const maxUploadBytes = 5 << 20 // 5 MB

var allowedImageExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true, ".avif": true,
}

// handleAdminUpload menerima multipart "file" dan menyimpannya ke UploadDir.
func (s *Server) handleAdminUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+512*1024)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		s.writeErr(w, http.StatusBadRequest, "berkas terlalu besar atau format salah (maks 5 MB)")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, "field 'file' wajib diisi")
		return
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if !allowedImageExt[ext] {
		s.writeErr(w, http.StatusBadRequest, "hanya gambar: jpg, png, webp, gif, avif")
		return
	}
	if !validImageHeader(f) {
		s.writeErr(w, http.StatusBadRequest, "isi berkas bukan gambar yang valid")
		return
	}

	name := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), randToken(6), ext)
	if err := os.MkdirAll(s.cfg.UploadDir, 0o755); err != nil {
		s.writeErr(w, http.StatusInternalServerError, "gagal membuat folder unggahan")
		return
	}
	dst := filepath.Join(s.cfg.UploadDir, name)
	out, err := os.Create(dst)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "gagal menyimpan berkas")
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(f, maxUploadBytes)); err != nil {
		os.Remove(dst)
		s.writeErr(w, http.StatusInternalServerError, "gagal menulis berkas")
		return
	}

	url := "/uploads/" + name
	if s.cfg.PublicBaseURL != "" {
		url = s.cfg.PublicBaseURL + url
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "url": url, "nama": name, "ukuran": hdr.Size,
	})
}

// validImageHeader memeriksa magic bytes.
func validImageHeader(f multipart.File) bool {
	head := make([]byte, 12)
	n, err := f.Read(head)
	if err != nil && n == 0 {
		return false
	}
	// kembalikan posisi baca
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false
	}
	if n >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF {
		return true // jpeg
	}
	if n >= 8 && string(head[0:8]) == "\x89PNG\r\n\x1a\n" {
		return true // png
	}
	if n >= 6 && (string(head[0:6]) == "GIF87a" || string(head[0:6]) == "GIF89a") {
		return true // gif
	}
	if n >= 12 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP" {
		return true // webp
	}
	if n >= 12 && string(head[4:8]) == "ftyp" &&
		(string(head[8:12]) == "avif" || string(head[8:12]) == "avis") {
		return true // avif
	}
	return false
}