package scraper

import (
	"testing"
	"time"
)

func TestRobotsAllowed(t *testing.T) {
	txt := `
# contoh
User-agent: *
Disallow: /admin/
Allow: /admin/public/
Disallow: /*.pdf$
Crawl-delay: 2.5

User-agent: OtherBot
Disallow:

User-agent: GamePortBot
Disallow: /private-only/
Allow: /
`
	f := parseRobots(txt)

	// Grup GamePortBot lebih spesifik, jadi aturan /admin/ dan *.pdf milik "*"
	// tidak berlaku untuk kita.
	bot := []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/games/celeste/", true},
		{"/admin/", true},
		{"/admin/public/x", true},
		{"/doc.pdf", true},
		{"/private-only/x", false},
	}
	for _, c := range bot {
		if got := f.allowed(c.path, "GamePortBot/0.1"); got != c.want {
			t.Errorf("GamePortBot %s: got %v want %v", c.path, got, c.want)
		}
	}

	// UA lain memakai grup "*".
	umum := []struct {
		path string
		want bool
	}{
		{"/admin/", false},
		{"/admin/public/x", true},
		{"/doc.pdf", false},
		{"/doc.pdf?x=1", true},
		{"/private-only/x", true},
	}
	for _, c := range umum {
		if got := f.allowed(c.path, "CrawlerX"); got != c.want {
			t.Errorf("CrawlerX %s: got %v want %v", c.path, got, c.want)
		}
	}

	// Grup tanpa aturan (Disallow kosong) berarti boleh semua.
	if !f.allowed("/apa-saja", "OtherBot") {
		t.Error("Disallow kosong seharusnya boleh semua")
	}

	if d := f.crawlDelay("CrawlerX"); d != 2500*time.Millisecond {
		t.Errorf("crawl-delay: got %v want 2.5s", d)
	}
	if d := f.crawlDelay("GamePortBot"); d != 0 {
		t.Errorf("grup GamePortBot tidak punya crawl-delay, got %v", d)
	}
}

func TestMatchRobots(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"/admin/", "/admin/", true},
		{"/admin/", "/admin/x", true},
		{"/admin", "/administrasi", true}, // pola tanpa "$" adalah awalan
		{"/admin$", "/admin", true},
		{"/admin$", "/admin/x", false},
		{"/*.pdf$", "/a/b.pdf", true},
		{"/*.pdf$", "/a/b.pdf?x=1", false},
		{"/*?sort=", "/port/x?sort=baru", true},
		{"/*?sort=", "/port/x", false},
		{"/", "/apa-saja", true},
	}
	for _, c := range cases {
		if got := matchRobots(c.pattern, c.path); got != c.want {
			t.Errorf("matchRobots(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
