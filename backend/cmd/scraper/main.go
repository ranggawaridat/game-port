// Command scraper mengambil konten baru dari sumber yang terdaftar di tabel
// scrape_sources dan menyimpannya sebagai draf port untuk ditinjau di admin.
//
//	DATABASE_URL=... go run ./cmd/scraper -dry-run
//	DATABASE_URL=... go run ./cmd/scraper -only droidports
//
// Hasil selalu published=false. Scraper menghormati robots.txt dan memberi
// jeda antar permintaan; lihat internal/scraper.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ranggawaridat/game-port/internal/config"
	"github.com/ranggawaridat/game-port/internal/db"
	"github.com/ranggawaridat/game-port/internal/scraper"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("scraper berhenti karena error", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	var (
		only      = flag.String("only", "", "hanya proses satu slug sumber (mis. droidports)")
		dryRun    = flag.Bool("dry-run", false, "tampilkan hasil tanpa menulis ke database")
		delay     = flag.Duration("delay", 1500*time.Millisecond, "jeda minimum antar permintaan ke host yang sama")
		timeout   = flag.Duration("timeout", 20*time.Minute, "batas waktu total satu kali jalan")
		userAgent = flag.String("user-agent", scraper.DefaultUserAgent, "User-Agent yang dipakai saat mengambil halaman")
		verbose   = flag.Bool("verbose", false, "log debug (termasuk halaman yang dilewati)")
	)
	flag.Parse()

	if *verbose {
		log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	if *delay < 0 {
		return fmt.Errorf("-delay tidak boleh negatif")
	}

	dsn, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	store, err := db.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer store.Close()

	s := &scraper.Scraper{
		Store:   store,
		Fetcher: scraper.NewFetcher(log, *userAgent, *delay),
		Log:     log,
		DryRun:  *dryRun,
	}

	results, err := s.Run(ctx, *only)
	if err != nil {
		return err
	}

	var totalBaru, totalGagal int
	gagal := false
	for _, r := range results {
		totalBaru += r.Created
		totalGagal += r.Failed
		if r.Err != nil {
			gagal = true
		}
	}
	log.Info("ringkasan scrape",
		"sumber", len(results), "draf_baru", totalBaru, "gagal", totalGagal, "dry_run", *dryRun)
	if gagal {
		return fmt.Errorf("sebagian sumber gagal; lihat log di atas")
	}
	return nil
}
