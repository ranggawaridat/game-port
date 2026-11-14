package api

import (
	"context"
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/ranggawaridat/game-port/internal/db"
)

// =============================================================================
// SEO — sitemap.xml
//
// Frontend publik tinggal di Vercel (gameport.web.id) dan backend di
// api.gameport.web.id. Agar sitemap selalu segar tanpa build step, backend
// membangkitkannya langsung dari database, lalu Vercel meneruskan
// /sitemap.xml ke sini lewat rewrite (lihat frontend/vercel.json).
//
// Daftar halaman yang didorong ke crawler:
//   /                          beranda + katalog
//   /port/{slug}               satu halaman per port terbit
//   /genre/{slug}              landing page long-tail
//   /platform/{slug}
//   /kategori/{slug}
// =============================================================================

const sitemapNS = "http://www.sitemaps.org/schemas/sitemap/0.9"

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// handleSitemap menyajikan /sitemap.xml dari isi database.
// Cache singkat di backend, lebih lama di edge Vercel.
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	ports, err := s.store.SitemapEntries(ctx)
	if err != nil {
		s.log.Error("sitemap: ports", "err", err)
		s.writeErr(w, http.StatusInternalServerError, "sitemap gagal dibangun")
		return
	}
	genres, err := s.store.GenreList(ctx)
	if err != nil {
		s.log.Error("sitemap: genre", "err", err)
		s.writeErr(w, http.StatusInternalServerError, "sitemap gagal dibangun")
		return
	}
	platforms, err := s.store.PlatformList(ctx)
	if err != nil {
		s.log.Error("sitemap: platform", "err", err)
		s.writeErr(w, http.StatusInternalServerError, "sitemap gagal dibangun")
		return
	}
	kategori, err := s.store.KategoriList(ctx)
	if err != nil {
		s.log.Error("sitemap: kategori", "err", err)
		s.writeErr(w, http.StatusInternalServerError, "sitemap gagal dibangun")
		return
	}

	body, err := sitemapBytes(s.cfg.PublicURL, ports, genres, platforms, kategori)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "sitemap gagal dibangun")
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600, s-maxage=3600")
	_, _ = w.Write(body)
}

// sitemapBytes menyusun dokumen sitemap. Dipisah dari handler supaya bisa
// diuji tanpa database.
func sitemapBytes(base string, ports []db.SitemapEntry, genres []db.Genre,
	platforms []db.Platform, kategori []db.Kategori) ([]byte, error) {

	base = strings.TrimRight(base, "/")
	set := sitemapSet{Xmlns: sitemapNS}
	set.URLs = append(set.URLs, sitemapURL{
		Loc: base + "/", ChangeFreq: "daily", Priority: "1.0",
	})

	for _, p := range ports {
		set.URLs = append(set.URLs, sitemapURL{
			Loc:        base + "/port/" + p.Slug,
			LastMod:    p.UpdatedAt.UTC().Format(time.RFC3339),
			ChangeFreq: "weekly",
			Priority:   "0.8",
		})
	}
	for _, g := range genres {
		if g.Jumlah == 0 {
			continue // halaman kosong tidak perlu diindeks
		}
		set.URLs = append(set.URLs, sitemapURL{
			Loc: base + "/genre/" + g.Slug, ChangeFreq: "weekly", Priority: "0.6",
		})
	}
	for _, p := range platforms {
		if p.Jumlah == 0 {
			continue
		}
		set.URLs = append(set.URLs, sitemapURL{
			Loc: base + "/platform/" + p.Slug, ChangeFreq: "weekly", Priority: "0.6",
		})
	}
	for _, k := range kategori {
		if k.Jumlah == 0 {
			continue
		}
		set.URLs = append(set.URLs, sitemapURL{
			Loc: base + "/kategori/" + k.Slug, ChangeFreq: "weekly", Priority: "0.6",
		})
	}

	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}
