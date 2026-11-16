package scraper

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/ranggawaridat/game-port/internal/db"
	"golang.org/x/net/html"
)

// collect mengambil item dari satu sumber sesuai kind di tabel scrape_sources.
func (s *Scraper) collect(ctx context.Context, src db.ScrapeSource) ([]Item, error) {
	switch src.Kind {
	case "rss":
		return s.collectRSS(ctx, src)
	case "playported":
		return s.collectPlayported(ctx, src)
	case "markdown":
		return s.collectMarkdown(ctx, src)
	default:
		return nil, fmt.Errorf("kind %q belum didukung", src.Kind)
	}
}

func srcLimit(src db.ScrapeSource) int {
	if src.MaxItems > 0 {
		return src.MaxItems
	}
	return 20
}

// ---------- RSS ----------

type rssDoc struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
}

// collectRSS membaca feed. Sumber berbasis feed adalah cara paling sopan untuk
// memantau konten baru, karena situs sendiri yang menyediakan ringkasannya.
func (s *Scraper) collectRSS(ctx context.Context, src db.ScrapeSource) ([]Item, error) {
	body, err := s.Fetcher.Get(ctx, src.URL)
	if err != nil {
		return nil, err
	}
	items, err := parseRSSItems(body, srcLimit(src))
	if err != nil {
		return nil, fmt.Errorf("parse RSS %s: %w", src.URL, err)
	}

	// Halaman detail biasanya memuat data terstruktur (JSON-LD/Open Graph)
	// yang tidak ada di feed: gambar sampul, deskripsi, dan porter.
	for i := range items {
		s.enrich(ctx, &items[i])
	}
	return items, nil
}

// parseRSSItems adalah bagian murni (tanpa jaringan) dari collectRSS.
func parseRSSItems(body []byte, limit int) ([]Item, error) {
	var doc rssDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	items := make([]Item, 0, limit)
	for _, raw := range doc.Channel.Items {
		if len(items) >= limit {
			break
		}
		title, version := cleanTitle(raw.Title)
		if title == "" {
			continue
		}
		link := strings.TrimSpace(raw.Link)
		id := strings.TrimSpace(raw.GUID)
		if id == "" {
			id = link
		}
		if id == "" {
			continue
		}
		item := Item{
			ExternalID:  id,
			Title:       title,
			Version:     version,
			Description: bersihTeks(raw.Description),
			SourceURL:   link,
			Platforms:   []string{"android"},
		}
		item.NeedsOwnData = butuhDataAsli(item.Description)
		items = append(items, item)
	}
	return items, nil
}

// ---------- PlayPorted ----------

// collectPlayported membaca halaman daftar PlayPorted dan hanya mengambil
// kartu yang menyatakan dukungan Android (data-runs_on).
func (s *Scraper) collectPlayported(ctx context.Context, src db.ScrapeSource) ([]Item, error) {
	body, err := s.Fetcher.Get(ctx, src.URL)
	if err != nil {
		return nil, err
	}
	doc, err := parseHTML(body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML %s: %w", src.URL, err)
	}

	items := parsePlayportedItems(doc, src.URL, srcLimit(src))
	for i := range items {
		s.enrich(ctx, &items[i])
	}
	return items, nil
}

// parsePlayportedItems adalah bagian murni (tanpa jaringan) dari collectPlayported.
func parsePlayportedItems(doc *html.Node, base string, limit int) []Item {
	cards := findAll(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "article" && hasClass(n, "card")
	})

	items := make([]Item, 0, limit)
	for _, card := range cards {
		if len(items) >= limit {
			break
		}
		if !strings.Contains(strings.ToLower(nodeAttr(card, "data-runs_on")), "android") {
			continue
		}

		titleEl := firstNode(card, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "h3" && hasClass(n, "title")
		})
		var href, title string
		if titleEl != nil {
			if a := firstNode(titleEl, func(n *html.Node) bool {
				return n.Type == html.ElementNode && n.Data == "a"
			}); a != nil {
				href = absolut(base, nodeAttr(a, "href"))
			}
			title = nodeText(titleEl)
		}
		if raw := strings.TrimSpace(nodeAttr(card, "data-game")); raw != "" {
			title = raw
		}
		title, version := cleanTitle(title)
		if title == "" || href == "" {
			continue
		}

		item := Item{
			ExternalID: href,
			Title:      title,
			Version:    version,
			SourceURL:  href,
		}

		// Nama proyek port (mis. "fallout1-ce") lebih berguna sebagai penulis
		// daripada nama penerbit game aslinya.
		if proj := firstNode(card, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "p" && hasClass(n, "proj")
		}); proj != nil {
			item.Author = nodeText(proj)
		}

		if img := firstNode(card, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "img" && hasClass(n, "cover")
		}); img != nil {
			item.ImageURL = absolut(base, nodeAttr(img, "src"))
		}

		// Daftar platform ada di <p class="os">, dipisah "•".
		var plat []string
		if osEl := firstNode(card, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "p" && hasClass(n, "os")
		}); osEl != nil {
			for _, p := range strings.Split(nodeText(osEl), "•") {
				plat = append(plat, p)
			}
		}
		item.Platforms = platformSlugs(plat...)

		items = append(items, item)
	}
	return items
}

// ---------- Markdown (awesome-list di GitHub) ----------

// collectMarkdown membaca tabel Markdown tiga kolom: Game | Description | Author.
func (s *Scraper) collectMarkdown(ctx context.Context, src db.ScrapeSource) ([]Item, error) {
	body, err := s.Fetcher.Get(ctx, src.URL)
	if err != nil {
		return nil, err
	}
	return parseMarkdownItems(string(body), srcLimit(src)), nil
}

// parseMarkdownItems adalah bagian murni (tanpa jaringan) dari collectMarkdown.
func parseMarkdownItems(text string, limit int) []Item {
	items := make([]Item, 0, limit)
	for _, cells := range markdownRows(text) {
		if len(items) >= limit {
			break
		}
		if len(cells) < 3 {
			continue
		}
		title, link := parseMDLink(cells[0])
		if title == "" || link == "" {
			continue
		}
		title, version := cleanTitle(title)
		if title == "" {
			continue
		}
		desc := bersihTeks(cells[1])
		author, _ := parseMDLink(cells[2])
		items = append(items, Item{
			ExternalID:   link,
			Title:        title,
			Version:      version,
			Description:  desc,
			Author:       author,
			SourceURL:    link,
			Platforms:    []string{"android"},
			NeedsOwnData: butuhDataAsli(desc),
		})
	}
	return items
}

// ---------- Pengayaan dari halaman detail ----------

// enrich melengkapi item dari halaman detail: deskripsi, gambar, porter, dan
// platform tambahan. Kegagalan di sini tidak fatal — data dari feed tetap dipakai.
func (s *Scraper) enrich(ctx context.Context, it *Item) {
	if it.SourceURL == "" {
		return
	}
	body, err := s.Fetcher.Get(ctx, it.SourceURL)
	if err != nil {
		s.Log.Debug("pengayaan dilewati", "url", it.SourceURL, "err", err)
		return
	}
	doc, err := parseHTML(body)
	if err != nil {
		return
	}
	nodes := jsonLDNodes(doc)

	var desc, author, image string
	var plats []string
	for _, n := range nodes {
		t := jsonType(n)
		if t["VideoGame"] || t["SoftwareApplication"] {
			if desc == "" {
				desc = jsonString(n, "description")
			}
			if author == "" {
				author = jsonNames(n["author"])
			}
			if image == "" {
				image = jsonFirstString(n["image"])
			}
			plats = append(plats, jsonPlatforms(n)...)
		}
	}
	if desc == "" {
		for _, n := range nodes {
			if jsonType(n)["WebPage"] {
				if d := jsonString(n, "description"); d != "" {
					desc = d
					break
				}
			}
		}
	}
	if desc == "" {
		desc = metaContent(doc, "og:description")
	}
	if image == "" {
		image = metaContent(doc, "og:image")
	}

	if d := bersihTeks(desc); d != "" {
		// Deskripsi halaman detail biasanya lebih menjelaskan daripada ringkasan
		// feed ("build baru ... sudah keluar"), jadi pilih yang lebih panjang.
		if it.Description == "" || len(d) > len(it.Description) {
			it.Description = d
		}
	}
	if it.Author == "" {
		it.Author = author
	}
	if it.ImageURL == "" && image != "" {
		it.ImageURL = absolut(it.SourceURL, image)
	}
	if len(plats) > 0 {
		it.Platforms = platformSlugs(append(plats, "android")...)
	}
	if !it.NeedsOwnData {
		it.NeedsOwnData = butuhDataAsli(it.Description)
	}
}

func jsonFirstString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	case map[string]any:
		return jsonString(t, "url")
	}
	return ""
}

// jsonPlatforms membaca gamePlatform/operatingSystem yang bisa berupa string
// atau array, lalu memecah nilai gabungan seperti "PC / PlayStation 2".
func jsonPlatforms(n map[string]any) []string {
	var out []string
	for _, key := range []string{"gamePlatform", "operatingSystem"} {
		switch v := n[key].(type) {
		case string:
			out = append(out, splitPlatforms(v)...)
		case []any:
			for _, e := range v {
				if s, ok := e.(string); ok {
					out = append(out, splitPlatforms(s)...)
				}
			}
		}
	}
	return out
}

func splitPlatforms(s string) []string {
	var out []string
	f := strings.FieldsFunc(s, func(r rune) bool {
		return r == '/' || r == ',' || r == '|' || r == '•' || r == '&'
	})
	for _, p := range f {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
