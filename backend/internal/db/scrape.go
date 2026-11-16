package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ScrapeSource adalah satu baris tabel scrape_sources: sumber yang di-scrape
// oleh cmd/scraper. Kategori dan status dipakai sebagai nilai awal draf.
type ScrapeSource struct {
	Slug         string
	Name         string
	Kind         string
	URL          string
	Enabled      bool
	CategorySlug string
	StatusSlug   string
	MaxItems     int
	LastRunAt    *time.Time
	LastStatus   string
	LastMessage  string
}

// ScrapeSources mengambil sumber yang aktif. Kalau only tidak kosong, hanya
// sumber itu yang diambil (termasuk yang sedang disabled, untuk uji manual).
func (s *Store) ScrapeSources(ctx context.Context, only string) ([]ScrapeSource, error) {
	q := `SELECT slug, name, kind, url, enabled, category_slug, status_slug,
	             max_items, last_run_at, last_status, last_message
	      FROM scrape_sources`
	args := []any{}
	if strings.TrimSpace(only) != "" {
		q += ` WHERE slug = $1`
		args = append(args, strings.TrimSpace(only))
	} else {
		q += ` WHERE enabled = true`
	}
	q += ` ORDER BY slug`

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("daftar sumber scrape: %w", err)
	}
	defer rows.Close()

	out := []ScrapeSource{}
	for rows.Next() {
		var src ScrapeSource
		if err := rows.Scan(&src.Slug, &src.Name, &src.Kind, &src.URL, &src.Enabled,
			&src.CategorySlug, &src.StatusSlug, &src.MaxItems, &src.LastRunAt,
			&src.LastStatus, &src.LastMessage); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

// ScrapeItemIDs mengembalikan external_id yang sudah pernah diambil dari
// sebuah sumber, supaya cron tidak membuat port yang sama berulang kali.
func (s *Store) ScrapeItemIDs(ctx context.Context, sourceSlug string) (map[string]bool, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT external_id FROM scrape_items WHERE source_slug = $1`, sourceSlug)
	if err != nil {
		return nil, fmt.Errorf("daftar item scrape: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		seen[id] = true
	}
	return seen, rows.Err()
}

// CatatScrapeItem menandai satu item sudah diproses. portID boleh nil bila
// item dilewati karena port dengan judul sama sudah ada.
func (s *Store) CatatScrapeItem(ctx context.Context, sourceSlug, externalID, title, url string, portID *int64) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO scrape_items (source_slug, external_id, title, url, port_id)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (source_slug, external_id) DO NOTHING`,
		sourceSlug, externalID, title, url, portID)
	if err != nil {
		return fmt.Errorf("catat item scrape: %w", err)
	}
	return nil
}

// PortAdaDenganJudul dipakai untuk dedupe antar-sumber: port dengan judul
// yang sama (tanpa peduli huruf besar/kecil) tidak dibuat dua kali.
func (s *Store) PortAdaDenganJudul(ctx context.Context, title string) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ports WHERE lower(game) = lower($1))`,
		strings.TrimSpace(title)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("cek judul port: %w", err)
	}
	return exists, nil
}

// PortAdaDenganSumber mengecek source_url, menangkap kasus judul berubah
// tetapi URL sumbernya sama.
func (s *Store) PortAdaDenganSumber(ctx context.Context, sourceURL string) (bool, error) {
	if strings.TrimSpace(sourceURL) == "" {
		return false, nil
	}
	var exists bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ports WHERE source_url = $1)`, sourceURL).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("cek sumber port: %w", err)
	}
	return exists, nil
}

// UpdateScrapeSource menyimpan hasil terakhir sebuah sumber untuk ditampilkan
// di log/operasional.
func (s *Store) UpdateScrapeSource(ctx context.Context, slug, status, message string) error {
	if len(message) > 500 {
		message = message[:500]
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE scrape_sources
		SET last_run_at = now(), last_status = $2, last_message = $3
		WHERE slug = $1`, slug, status, message)
	if err != nil {
		return fmt.Errorf("update sumber scrape: %w", err)
	}
	return nil
}
