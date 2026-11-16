package scraper

import (
	"bytes"
	"encoding/json"
	stdhtml "html"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Item adalah satu hasil scraping yang siap disimpan sebagai draf port.
type Item struct {
	ExternalID   string
	Title        string
	Version      string
	Description  string
	Author       string
	ImageURL     string
	SourceURL    string
	Platforms    []string
	NeedsOwnData bool
}

var (
	versionLike = regexp.MustCompile(`^(?:v?\d|build[-.]|continuous$|nightly$|git-|android-)`)
	schemeRe    = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`)
	spaceRe     = regexp.MustCompile(`\s+`)
	mdLinkRe    = regexp.MustCompile(`^\[([^\]]*)\]\((\S+)\)$`)
	tagRe       = regexp.MustCompile(`<[^>]+>`)
)

// cleanTitle memisahkan versi dari judul. Contoh:
//
//	"Celeste (v1.1.0)"                    -> "Celeste", "v1.1.0"
//	"Pokémon ... (Gen1Recomp) (v0.4.7)"   -> "Pokémon ... (Gen1Recomp)", "v0.4.7"
//	"Celeste (Android Port)"              -> "Celeste", ""
func cleanTitle(raw string) (title, version string) {
	t := strings.Join(strings.Fields(stdhtml.UnescapeString(raw)), " ")
	lower := strings.ToLower(t)
	if strings.HasPrefix(lower, "new port:") {
		t = strings.TrimSpace(t[len("new port:"):])
	}

	if i := strings.LastIndex(t, " ("); i > 0 && strings.HasSuffix(t, ")") {
		inner := strings.TrimSpace(t[i+2 : len(t)-1])
		if versionLike.MatchString(strings.ToLower(inner)) {
			version = inner
			t = strings.TrimSpace(t[:i])
		}
	}
	for _, suf := range []string{" (Android Port)", " (Android)"} {
		if strings.HasSuffix(t, suf) {
			t = strings.TrimSpace(strings.TrimSuffix(t, suf))
		}
	}
	return t, version
}

// bersihTeks membuang tag HTML dan merapikan spasi.
func bersihTeks(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = stdhtml.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// butuhDataAsli menebak apakah port memerlukan berkas game milik pengguna.
func butuhDataAsli(desc string) bool {
	d := strings.ToLower(desc)
	for _, kunci := range []string{
		"own copy", "own game", "own data", "your own", "game files", "game data",
		"retail", "you supply", "bring your own", "original game", "original files",
		"requires your", "requires the original", "needs the original", "copy of the game",
		// padanan Indonesia yang sering dipakai di deskripsi lokal
		"milik sendiri", "salinan game", "berkas game", "data game", "game asli",
		"butuh salinan", "membawa sendiri",
	} {
		if strings.Contains(d, kunci) {
			return true
		}
	}
	return false
}

// platformSlug memetakan nama platform bebas ke slug di tabel platforms.
func platformSlug(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "android":
		return "android"
	case "windows", "pc", "microsoft windows":
		return "windows"
	case "linux":
		return "linux"
	case "macos", "mac os", "mac", "osx", "os x":
		return "macos"
	case "ios", "iphone", "ipad":
		return "ios"
	case "steam deck", "steamdeck":
		return "steam-deck"
	case "web", "browser":
		return "web"
	case "quest", "meta quest", "vr":
		return "quest"
	case "switch", "nintendo switch":
		return "switch"
	}
	return ""
}

// platformSlugs memetakan daftar nama platform dan selalu menyertakan android,
// karena katalog ini hanya memuat port yang berjalan di Android.
func platformSlugs(names ...string) []string {
	seen := map[string]bool{"android": true}
	out := []string{"android"}
	for _, n := range names {
		if sl := platformSlug(n); sl != "" && !seen[sl] {
			seen[sl] = true
			out = append(out, sl)
		}
	}
	return out
}

// ---------- HTML ----------

func parseHTML(body []byte) (*html.Node, error) {
	return html.Parse(bytes.NewReader(body))
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(stdhtml.UnescapeString(b.String())), " ")
}

// findAll mengumpulkan node yang lolos predikat (pre-order).
func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if match(x) {
			out = append(out, x)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func elementWithClass(n *html.Node, tag, class string) *html.Node {
	return firstNode(n, func(x *html.Node) bool {
		return x.Type == html.ElementNode && strings.EqualFold(x.Data, tag) &&
			hasClass(x, class)
	})
}

func firstNode(n *html.Node, match func(*html.Node) bool) *html.Node {
	found := findAll(n, match)
	if len(found) == 0 {
		return nil
	}
	return found[0]
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(nodeAttr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func metaContent(doc *html.Node, key string) string {
	n := firstNode(doc, func(x *html.Node) bool {
		if x.Type != html.ElementNode || x.Data != "meta" {
			return false
		}
		k := nodeAttr(x, "property")
		if k == "" {
			k = nodeAttr(x, "name")
		}
		return strings.EqualFold(k, key)
	})
	if n == nil {
		return ""
	}
	return strings.TrimSpace(nodeAttr(n, "content"))
}

// ---------- JSON-LD ----------

// jsonLDNodes mengurai semua <script type="application/ld+json"> dan
// mengembalikan node tingkat atas (termasuk isi @graph).
func jsonLDNodes(doc *html.Node) []map[string]any {
	scripts := findAll(doc, func(x *html.Node) bool {
		return x.Type == html.ElementNode && x.Data == "script" &&
			strings.EqualFold(nodeAttr(x, "type"), "application/ld+json")
	})
	var out []map[string]any
	for _, s := range scripts {
		var v any
		if err := json.Unmarshal([]byte(nodeTextRaw(s)), &v); err != nil {
			continue
		}
		out = append(out, flattenJSONLD(v)...)
	}
	return out
}

// nodeTextRaw mengambil isi script apa adanya (tanpa merapikan spasi, karena
// JSON tidak peduli tetapi JSON-LD bisa memuat spasi penting di dalam string).
func nodeTextRaw(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}

func flattenJSONLD(v any) []map[string]any {
	switch t := v.(type) {
	case []any:
		var out []map[string]any
		for _, e := range t {
			out = append(out, flattenJSONLD(e)...)
		}
		return out
	case map[string]any:
		if g, ok := t["@graph"].([]any); ok {
			// Pembungkus @graph tidak membawa data; isinya yang dipakai.
			return flattenJSONLD(g)
		}
		return []map[string]any{t}
	}
	return nil
}

func jsonType(n map[string]any) map[string]bool {
	set := map[string]bool{}
	switch t := n["@type"].(type) {
	case string:
		set[t] = true
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				set[s] = true
			}
		}
	}
	return set
}

func jsonString(n map[string]any, key string) string {
	if s, ok := n[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func jsonNames(v any) string {
	var names []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if s := jsonString(t, "name"); s != "" {
				names = append(names, s)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case string:
			if s := strings.TrimSpace(t); s != "" {
				names = append(names, s)
			}
		}
	}
	walk(v)
	return strings.Join(uniqueStrings(names), ", ")
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// ---------- Markdown ----------

// markdownRows mengembalikan baris tabel Markdown sebagai potongan sel.
// Baris separator (|---|---|) dilewati.
func markdownRows(text string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := splitTableRow(line)
		if len(cells) == 0 || isSeparatorRow(cells) {
			continue
		}
		rows = append(rows, cells)
	}
	return rows
}

func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	var cells []string
	var cur strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(cur.String()))
	return cells
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		c = strings.TrimSpace(strings.Trim(c, ":"))
		if c == "" || strings.Trim(c, "-") != "" {
			return false
		}
	}
	return true
}

// parseMDLink mengurai "[teks](url)" menjadi teks dan URL.
func parseMDLink(s string) (text, url string) {
	m := mdLinkRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return bersihTeks(s), ""
	}
	return strings.TrimSpace(bersihTeks(m[1])), strings.TrimSpace(m[2])
}

func absolut(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || schemeRe.MatchString(ref) {
		return ref
	}
	if strings.HasPrefix(ref, "//") {
		return "https:" + ref
	}
	b := strings.TrimRight(base, "/")
	if strings.HasPrefix(ref, "/") {
		// buang path dari base
		if i := strings.Index(b, "://"); i >= 0 {
			rest := b[i+3:]
			if j := strings.IndexByte(rest, '/'); j >= 0 {
				b = b[:i+3+j]
			}
		}
		return b + ref
	}
	return b + "/" + ref
}
