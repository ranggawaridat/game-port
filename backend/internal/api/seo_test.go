package api

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/ranggawaridat/game-port/internal/db"
)

func TestSitemapBytes(t *testing.T) {
	ubah := time.Date(2026, 11, 14, 10, 30, 0, 0, time.UTC)
	body, err := sitemapBytes(
		"https://gameport.web.id/",
		[]db.SitemapEntry{{Slug: "doom", UpdatedAt: ubah}},
		[]db.Genre{{Slug: "action", Name: "Action", Jumlah: 3}, {Slug: "mmo", Name: "MMO", Jumlah: 0}},
		[]db.Platform{{Slug: "android", Name: "Android", Jumlah: 1}},
		[]db.Kategori{{Slug: "source-port", Name: "Source Port", Jumlah: 1}, {Slug: "classic", Name: "Klasik", Jumlah: 0}},
	)
	if err != nil {
		t.Fatalf("sitemapBytes: %v", err)
	}

	teks := string(body)
	for _, wajib := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`,
		`<loc>https://gameport.web.id/</loc>`,
		`<loc>https://gameport.web.id/port/doom</loc>`,
		`<lastmod>2026-11-14T10:30:00Z</lastmod>`,
		`<loc>https://gameport.web.id/genre/action</loc>`,
		`<loc>https://gameport.web.id/platform/android</loc>`,
		`<loc>https://gameport.web.id/kategori/source-port</loc>`,
	} {
		if !strings.Contains(teks, wajib) {
			t.Errorf("sitemap tidak memuat %q\n%s", wajib, teks)
		}
	}
	for _, terlarang := range []string{"/genre/mmo", "/kategori/classic", "gameport.web.id//"} {
		if strings.Contains(teks, terlarang) {
			t.Errorf("sitemap seharusnya tidak memuat %q", terlarang)
		}
	}

	var cek struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(body, &cek); err != nil {
		t.Fatalf("XML tidak valid: %v", err)
	}
}
