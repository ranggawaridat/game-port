package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ranggawaridat/game-port/internal/db"
)

// Scraper menjalankan pengambilan konten dari tabel scrape_sources dan
// menyimpannya sebagai draf port (published=false).
type Scraper struct {
	Store   *db.Store
	Fetcher *Fetcher
	Log     *slog.Logger
	DryRun  bool
}

// Result ringkasan satu sumber.
type Result struct {
	Slug    string
	Fetched int
	Created int
	Skipped int
	Failed  int
	Err     error
}

// Run memproses semua sumber aktif, atau hanya satu bila `only` diisi.
func (s *Scraper) Run(ctx context.Context, only string) ([]Result, error) {
	srcs, err := s.Store.ScrapeSources(ctx, only)
	if err != nil {
		return nil, err
	}
	if len(srcs) == 0 {
		return nil, fmt.Errorf("tidak ada sumber scraping yang cocok")
	}

	results := make([]Result, 0, len(srcs))
	for _, src := range srcs {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		results = append(results, s.runSource(ctx, src))
	}
	return results, nil
}

func (s *Scraper) runSource(ctx context.Context, src db.ScrapeSource) Result {
	res := Result{Slug: src.Slug}
	s.Log.Info("mulai scrape", "sumber", src.Slug, "kind", src.Kind, "url", src.URL)

	items, err := s.collect(ctx, src)
	if err != nil {
		res.Err = err
		s.updateStatus(ctx, src.Slug, "error", err.Error())
		s.Log.Error("scrape gagal", "sumber", src.Slug, "err", err)
		return res
	}
	res.Fetched = len(items)

	seen, err := s.Store.ScrapeItemIDs(ctx, src.Slug)
	if err != nil {
		res.Err = err
		s.updateStatus(ctx, src.Slug, "error", err.Error())
		s.Log.Error("baca riwayat item gagal", "sumber", src.Slug, "err", err)
		return res
	}

	for _, it := range items {
		if it.ExternalID == "" || strings.TrimSpace(it.Title) == "" {
			res.Skipped++
			continue
		}
		if seen[it.ExternalID] {
			res.Skipped++
			continue
		}

		dup, err := s.sudahAda(ctx, it)
		if err != nil {
			res.Failed++
			s.Log.Error("cek duplikat gagal", "judul", it.Title, "err", err)
			continue
		}
		if dup {
			res.Skipped++
			if !s.DryRun {
				if err := s.Store.CatatScrapeItem(ctx, src.Slug, it.ExternalID, it.Title, it.SourceURL, nil); err != nil {
					s.Log.Warn("catat item duplikat gagal", "judul", it.Title, "err", err)
				}
			}
			s.Log.Debug("dilewati, sudah ada", "sumber", src.Slug, "judul", it.Title)
			continue
		}

		if s.DryRun {
			res.Created++
			s.Log.Info("[dry-run] calon draf", "sumber", src.Slug, "judul", it.Title,
				"versi", it.Version, "penulis", it.Author, "gambar", it.ImageURL != "")
			continue
		}

		port, err := s.Store.CreatePort(ctx, kePortInput(src, it))
		if err != nil {
			res.Failed++
			s.Log.Error("simpan draf gagal", "sumber", src.Slug, "judul", it.Title, "err", err)
			continue
		}
		id := port.ID
		if err := s.Store.CatatScrapeItem(ctx, src.Slug, it.ExternalID, it.Title, it.SourceURL, &id); err != nil {
			s.Log.Warn("catat item gagal", "judul", it.Title, "err", err)
		}
		seen[it.ExternalID] = true
		res.Created++
		s.Log.Info("draf dibuat", "sumber", src.Slug, "judul", it.Title,
			"slug", port.Slug, "id", port.ID)
	}

	status := "ok"
	if res.Failed > 0 {
		status = "sebagian"
	}
	msg := fmt.Sprintf("%d item, %d draf baru, %d dilewati, %d gagal",
		res.Fetched, res.Created, res.Skipped, res.Failed)
	s.updateStatus(ctx, src.Slug, status, msg)
	s.Log.Info("selesai scrape", "sumber", src.Slug, "status", status, "pesan", msg)
	return res
}

// sudahAda mencegah duplikat lintas sumber: judul sama atau URL sumber sama.
func (s *Scraper) sudahAda(ctx context.Context, it Item) (bool, error) {
	dup, err := s.Store.PortAdaDenganJudul(ctx, it.Title)
	if err != nil || dup {
		return dup, err
	}
	return s.Store.PortAdaDenganSumber(ctx, it.SourceURL)
}

func (s *Scraper) updateStatus(ctx context.Context, slug, status, msg string) {
	if s.DryRun {
		return
	}
	if err := s.Store.UpdateScrapeSource(ctx, slug, status, msg); err != nil {
		s.Log.Warn("simpan status sumber gagal", "sumber", slug, "err", err)
	}
}

func kePortInput(src db.ScrapeSource, it Item) db.PortInput {
	desc := strings.TrimSpace(it.Description)
	if desc == "" {
		desc = "Port Android dari " + it.Title + "."
	}
	plats := it.Platforms
	if len(plats) == 0 {
		plats = []string{"android"}
	}
	in := db.PortInput{
		Game:         it.Title,
		Version:      strings.TrimSpace(it.Version),
		Description:  desc,
		Author:       strings.TrimSpace(it.Author),
		CategorySlug: src.CategorySlug,
		StatusSlug:   src.StatusSlug,
		Published:    false,
		NeedsOwnData: it.NeedsOwnData,
		Platforms:    plats,
	}
	if v := strings.TrimSpace(it.ImageURL); v != "" {
		in.ImageURL = &v
	}
	if v := strings.TrimSpace(it.SourceURL); v != "" {
		in.SourceURL = &v
	}
	return in
}
