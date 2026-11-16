package scraper

import "testing"

const rssContoh = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Contoh</title>
    <item>
      <title>Celeste (v1.1.0)</title>
      <link>https://droidports.com/games/celeste/</link>
      <guid isPermaLink="false">droidports:celeste@v1.1.0</guid>
      <description>A new build of the Celeste port (PC) is out: v1.1.0.</description>
    </item>
    <item>
      <title>New port: Discord Times</title>
      <link>https://droidports.com/games/discord-times/</link>
      <guid>droidports:discord-times#added</guid>
      <description>Discord Times (PC) just joined the index.</description>
    </item>
  </channel>
</rss>`

func TestParseRSSItems(t *testing.T) {
	items, err := parseRSSItems([]byte(rssContoh), 10)
	if err != nil {
		t.Fatalf("parseRSSItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("dapat %d item, want 2", len(items))
	}
	if items[0].Title != "Celeste" || items[0].Version != "v1.1.0" {
		t.Errorf("item 0 = %q / %q", items[0].Title, items[0].Version)
	}
	if items[0].ExternalID != "droidports:celeste@v1.1.0" {
		t.Errorf("external id = %q", items[0].ExternalID)
	}
	if items[1].Title != "Discord Times" {
		t.Errorf("item 1 judul = %q", items[1].Title)
	}

	// Batas jumlah dihormati.
	if got, _ := parseRSSItems([]byte(rssContoh), 1); len(got) != 1 {
		t.Errorf("limit 1 menghasilkan %d item", len(got))
	}
}

const htmlPlayported = `<html><body>
<article class="card" data-runs_on="windows linux android" data-game="Fallout: A Post Nuclear Role Playing Game">
  <img class="cover sm" src="https://images.example/co1.jpg">
  <h3 class="title"><a href="/ports/303-fallout/">Fallout: A Post Nuclear Role Playing Game</a></h3>
  <p class="proj">fallout1-ce</p>
  <p class="os">Windows • macOS • Linux • Android • iOS</p>
</article>
<article class="card" data-runs_on="windows only" data-game="Game Tanpa Android">
  <h3 class="title"><a href="/ports/999-pc/">Game Tanpa Android</a></h3>
</article>
</body></html>`

func TestParsePlayportedItems(t *testing.T) {
	doc, err := parseHTML([]byte(htmlPlayported))
	if err != nil {
		t.Fatalf("parseHTML: %v", err)
	}
	items := parsePlayportedItems(doc, "https://playported.com/runs-on/android/", 10)
	if len(items) != 1 {
		t.Fatalf("dapat %d item, want 1 (hanya yang Android)", len(items))
	}
	it := items[0]
	if it.Title != "Fallout: A Post Nuclear Role Playing Game" {
		t.Errorf("judul = %q", it.Title)
	}
	if it.SourceURL != "https://playported.com/ports/303-fallout/" {
		t.Errorf("url = %q", it.SourceURL)
	}
	if it.Author != "fallout1-ce" {
		t.Errorf("author = %q", it.Author)
	}
	if it.ImageURL != "https://images.example/co1.jpg" {
		t.Errorf("gambar = %q", it.ImageURL)
	}
	want := "android,windows,macos,linux,ios"
	if got := join(items[0].Platforms); got != want {
		t.Errorf("platform = %q, want %q", got, want)
	}
}

func join(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func TestJSONLDNodes(t *testing.T) {
	halaman := `<html><head>
<script type="application/ld+json">
{"@context":"https://schema.org","@graph":[
 {"@type":["VideoGame","SoftwareApplication"],"name":"Celeste (Android Port)",
  "description":"Port native Celeste.","image":"https://droidports.com/og/celeste.png",
  "author":{"@type":"Organization","name":"BelmanteGu, KaliKylee"},
  "gamePlatform":["Android","PC"]},
 {"@type":"WebPage","description":"Deskripsi halaman."}
]}
</script>
<meta property="og:image" content="https://droidports.com/og/celeste.png">
</head><body></body></html>`

	doc, err := parseHTML([]byte(halaman))
	if err != nil {
		t.Fatalf("parseHTML: %v", err)
	}
	nodes := jsonLDNodes(doc)
	if len(nodes) != 2 {
		t.Fatalf("dapat %d node JSON-LD, want 2", len(nodes))
	}

	var game map[string]any
	for _, n := range nodes {
		if jsonType(n)["VideoGame"] {
			game = n
		}
	}
	if game == nil {
		t.Fatal("node VideoGame tidak ditemukan")
	}
	if got := jsonString(game, "description"); got != "Port native Celeste." {
		t.Errorf("deskripsi = %q", got)
	}
	if got := jsonNames(game["author"]); got != "BelmanteGu, KaliKylee" {
		t.Errorf("author = %q", got)
	}
	if got := jsonFirstString(game["image"]); got != "https://droidports.com/og/celeste.png" {
		t.Errorf("gambar = %q", got)
	}
	if got := jsonPlatforms(game); len(got) != 2 {
		t.Errorf("platform = %v", got)
	}
	if got := metaContent(doc, "og:image"); got != "https://droidports.com/og/celeste.png" {
		t.Errorf("og:image = %q", got)
	}
}
