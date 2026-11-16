package scraper

import (
	"strings"
	"testing"
)

func TestCleanTitle(t *testing.T) {
	cases := []struct {
		in, title, version string
	}{
		{"Celeste (v1.1.0)", "Celeste", "v1.1.0"},
		{"Crash Bandicoot (2.6)", "Crash Bandicoot", "2.6"},
		{"Pokémon Red, Gold & Ruby (Gen1Recomp) (v0.4.7)", "Pokémon Red, Gold & Ruby (Gen1Recomp)", "v0.4.7"},
		{"Celeste (Android Port)", "Celeste", ""},
		{"New port: Discord Times", "Discord Times", ""},
		{"Half-Life & GoldSrc Games (continuous)", "Half-Life & GoldSrc Games", "continuous"},
		{"  Spasi   berlebih  ", "Spasi berlebih", ""},
	}
	for _, c := range cases {
		title, version := cleanTitle(c.in)
		if title != c.title || version != c.version {
			t.Errorf("cleanTitle(%q) = (%q, %q), want (%q, %q)",
				c.in, title, version, c.title, c.version)
		}
	}
}

func TestButuhDataAsli(t *testing.T) {
	if !butuhDataAsli("You need your own copy of the game.") {
		t.Error("frasa 'own copy' harus terdeteksi")
	}
	if !butuhDataAsli("Requires the original game files.") {
		t.Error("frasa 'original game' harus terdeteksi")
	}
	if butuhDataAsli("Freeware standalone, no extra files needed.") {
		t.Error("freeware standalone tidak boleh butuh data asli")
	}
}

func TestPlatformSlugs(t *testing.T) {
	got := platformSlugs("PC", "macOS", "Android", "Steam Deck", "Tidak Dikenal")
	want := []string{"android", "windows", "macos", "steam-deck"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("platformSlugs = %v, want %v", got, want)
	}
}

func TestMarkdownItems(t *testing.T) {
	md := `
# Daftar

| Game | Description | Author |
|------|-------------|--------|
| [Aerofoil (Glider PRO)](https://github.com/elasota/Aerofoil) | Port klasik dengan APK Android. | [elasota](https://github.com/elasota) |
| [Barony](https://github.com/DifferentNet/barony-android-port) | Butuh salinan Barony milik sendiri. | [DifferentNet](https://github.com/DifferentNet) |
| bukan tabel |
`
	items := parseMarkdownItems(md, 10)
	if len(items) != 2 {
		t.Fatalf("dapat %d item, want 2", len(items))
	}
	if items[0].Title != "Aerofoil (Glider PRO)" {
		t.Errorf("judul = %q", items[0].Title)
	}
	if items[0].SourceURL != "https://github.com/elasota/Aerofoil" {
		t.Errorf("url = %q", items[0].SourceURL)
	}
	if items[0].Author != "elasota" {
		t.Errorf("author = %q", items[0].Author)
	}
	if items[1].NeedsOwnData != true {
		t.Error("Barony seharusnya butuh data asli")
	}
	if items[0].NeedsOwnData {
		t.Error("Aerofoil tidak butuh data asli")
	}
}

func TestAbsolut(t *testing.T) {
	cases := []struct{ base, ref, want string }{
		{"https://playported.com/runs-on/android/", "/ports/1-x/", "https://playported.com/ports/1-x/"},
		{"https://droidports.com/games/x/", "https://cdn.example/a.png", "https://cdn.example/a.png"},
		{"https://droidports.com/games/x/", "//cdn.example/a.png", "https://cdn.example/a.png"},
		{"https://droidports.com/rss.xml", "img.png", "https://droidports.com/rss.xml/img.png"},
	}
	for _, c := range cases {
		if got := absolut(c.base, c.ref); got != c.want {
			t.Errorf("absolut(%q,%q) = %q, want %q", c.base, c.ref, got, c.want)
		}
	}
}
