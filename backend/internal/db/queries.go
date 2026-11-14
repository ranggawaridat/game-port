package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound dikembalikan bila baris tidak ada.
var ErrNotFound = errors.New("tidak ditemukan")

// PortFilter adalah parameter pencarian publik.
type PortFilter struct {
	Q        string
	Kategori []string
	Status   []string
	Platform []string
	Genre    []string
	Own      *bool
	Sort     string
	Limit    int
	Offset   int
	// AdminOnly=false hanya menampilkan published=true
	AdminOnly bool
}

var sortSQL = map[string]string{
	"nama":      "game_sort ASC",
	"nama-desc": "game_sort DESC",
	"kategori":  "category_urutan ASC, game_sort ASC",
	"status":    "status_bobot DESC, game_sort ASC",
	"populer":   "view_count DESC, game_sort ASC",
	"terbaru":   "created_at DESC",
	"download":  "download_count DESC, game_sort ASC",
}

func buildWhere(f PortFilter) (string, []any) {
	var conds []string
	var args []any
	n := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if !f.AdminOnly {
		conds = append(conds, "published = true")
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		like := "%" + q + "%"
		conds = append(conds, fmt.Sprintf(
			"(game ILIKE %s OR description ILIKE %s OR author ILIKE %s)",
			n(like), n(like), n(like)))
	}
	if len(f.Kategori) > 0 {
		conds = append(conds, "category_slug = ANY("+n(f.Kategori)+")")
	}
	if len(f.Status) > 0 {
		conds = append(conds, "status_slug = ANY("+n(f.Status)+")")
	}
	if len(f.Genre) > 0 {
		conds = append(conds, "genre_slug = ANY("+n(f.Genre)+")")
	}
	if len(f.Platform) > 0 {
		conds = append(conds, "id IN (SELECT port_id FROM port_platforms WHERE platform_slug = ANY("+n(f.Platform)+"))")
	}
	if f.Own != nil {
		conds = append(conds, "needs_own_data = "+n(*f.Own))
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

const portColumns = `id, slug, game, game_sort, version, file_size, description,
	author, image_url, download_url, iso_url, source_url, install_notes,
	view_count, download_count,
	published, needs_own_data, created_at, updated_at,
	category_slug, category_name, category_urutan,
	status_slug, status_name, status_warna, status_bobot,
	genre_slug, genre_name, platforms::text[]`

func scanPort(row pgx.Row, p *Port) error {
	return row.Scan(
		&p.ID, &p.Slug, &p.Game, &p.GameSort, &p.Version, &p.FileSize,
		&p.Description, &p.Author, &p.ImageURL, &p.DownloadURL, &p.IsoURL,
		&p.SourceURL, &p.InstallNotes,
		&p.ViewCount, &p.DownloadCount, &p.Published, &p.NeedsOwnData,
		&p.CreatedAt, &p.UpdatedAt,
		&p.CategorySlug, &p.CategoryName, &p.CategoryOrder,
		&p.StatusSlug, &p.StatusName, &p.StatusColor, &p.StatusWeight,
		&p.GenreSlug, &p.GenreName, &p.Platforms,
	)
}

// ListPorts menjalankan pencarian berhalaman.
func (s *Store) ListPorts(ctx context.Context, f PortFilter) (*PortList, error) {
	where, args := buildWhere(f)

	var total int
	if err := s.Pool.QueryRow(ctx,
		"SELECT count(*) FROM v_port_detail"+where, args...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("hitung total: %w", err)
	}

	order, ok := sortSQL[f.Sort]
	if !ok {
		order = sortSQL["kategori"]
	}

	listArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	q := fmt.Sprintf("SELECT %s FROM v_port_detail%s ORDER BY %s LIMIT $%d OFFSET $%d",
		portColumns, where, order, len(args)+1, len(args)+2)

	rows, err := s.Pool.Query(ctx, q, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("query daftar: %w", err)
	}
	defer rows.Close()

	items := make([]Port, 0, f.Limit)
	for rows.Next() {
		var p Port
		if err := scanPort(rows, &p); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &PortList{Total: total, Limit: f.Limit, Offset: f.Offset, Items: items}, nil
}

// GetPort mengambil satu port beserta 6 terkait.
func (s *Store) GetPort(ctx context.Context, slug string, admin bool) (*PortDetail, error) {
	q := "SELECT " + portColumns + " FROM v_port_detail WHERE slug = $1"
	if !admin {
		q += " AND published = true"
	}
	var p Port
	if err := scanPort(s.Pool.QueryRow(ctx, q, slug), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT slug, game FROM v_port_detail
		WHERE category_slug = $1 AND slug <> $2 AND published = true
		ORDER BY game_sort LIMIT 6`, p.CategorySlug, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	related := make([]Ref, 0, 6)
	for rows.Next() {
		var r Ref
		if err := rows.Scan(&r.Slug, &r.Name); err != nil {
			return nil, err
		}
		related = append(related, r)
	}
	return &PortDetail{Port: p, Related: related}, rows.Err()
}

// Statistik dari view.
func (s *Store) Statistik(ctx context.Context) (*Statistik, error) {
	var st Statistik
	err := s.Pool.QueryRow(ctx, `
		SELECT total_port, terbit, total_author, stabil, beta, eksperimental,
		       alpha, total_view, total_download
		FROM v_statistik`).Scan(
		&st.TotalPort, &st.Terbit, &st.TotalAuthor, &st.Stabil,
		&st.Beta, &st.Eksperimental, &st.Alpha, &st.TotalView, &st.TotalDownld,
	)
	return &st, err
}

// KategoriList mengembalikan kategori + jumlah port terbit.
func (s *Store) KategoriList(ctx context.Context) ([]Kategori, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.slug, c.name, c.urutan, count(p.id)
		FROM categories c
		LEFT JOIN ports p ON p.category_slug = c.slug AND p.published = true
		GROUP BY c.slug, c.name, c.urutan ORDER BY c.urutan`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Kategori{}
	for rows.Next() {
		var k Kategori
		if err := rows.Scan(&k.Slug, &k.Name, &k.Urutan, &k.Jumlah); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// StatusList mengembalikan status + jumlah.
func (s *Store) StatusList(ctx context.Context) ([]Status, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT s.slug, s.name, s.warna, s.bobot, count(p.id)
		FROM statuses s
		LEFT JOIN ports p ON p.status_slug = s.slug AND p.published = true
		GROUP BY s.slug, s.name, s.warna, s.bobot ORDER BY s.bobot DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Status{}
	for rows.Next() {
		var st Status
		if err := rows.Scan(&st.Slug, &st.Name, &st.Warna, &st.Bobot, &st.Jumlah); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// PlatformList mengembalikan platform + jumlah.
func (s *Store) PlatformList(ctx context.Context) ([]Platform, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT pl.slug, pl.name, count(p.id)
		FROM platforms pl
		JOIN port_platforms pp ON pp.platform_slug = pl.slug
		JOIN ports p ON p.id = pp.port_id AND p.published = true
		GROUP BY pl.slug, pl.name ORDER BY count(p.id) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Platform{}
	for rows.Next() {
		var p Platform
		if err := rows.Scan(&p.Slug, &p.Name, &p.Jumlah); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GenreList mengembalikan genre + jumlah.
func (s *Store) GenreList(ctx context.Context) ([]Genre, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT g.slug, g.name, count(p.id)
		FROM genres g
		LEFT JOIN ports p ON p.genre_slug = g.slug AND p.published = true
		GROUP BY g.slug, g.name ORDER BY count(p.id) DESC, g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Genre{}
	for rows.Next() {
		var g Genre
		if err := rows.Scan(&g.Slug, &g.Name, &g.Jumlah); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ---------- Mutasi (admin) ----------

// Slugify membuat slug dari nama game.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "port"
	}
	return out
}

// uniqueSlug mencari slug yang belum dipakai (kecuali excludeID).
func (s *Store) uniqueSlug(ctx context.Context, base string, excludeID int64) (string, error) {
	slug := base
	for i := 2; i < 200; i++ {
		var exists bool
		err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ports WHERE slug = $1 AND id <> $2)`,
			slug, excludeID).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return "", errors.New("tidak bisa membuat slug unik")
}

// CreatePort menyimpan port baru.
func (s *Store) CreatePort(ctx context.Context, in PortInput) (*Port, error) {
	if strings.TrimSpace(in.Game) == "" {
		return nil, errors.New("nama game wajib diisi")
	}
	slug, err := s.uniqueSlug(ctx, Slugify(in.Game), 0)
	if err != nil {
		return nil, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO ports (slug, game, game_sort, version, file_size, genre_slug,
		                   description, author, category_slug, status_slug,
		                   image_url, download_url, iso_url, source_url, install_notes,
		                   published, needs_own_data)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING id`,
		slug, in.Game, strings.ToLower(in.Game), in.Version, in.FileSize,
		in.GenreSlug, in.Description, in.Author, in.CategorySlug, in.StatusSlug,
		in.ImageURL, in.DownloadURL, in.IsoURL, in.SourceURL, in.InstallNotes,
		in.Published, in.NeedsOwnData).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert port: %w", err)
	}

	if err := replacePlatforms(ctx, tx, id, in.Platforms); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getPortByID(ctx, id)
}

// UpdatePort mengubah port yang ada.
func (s *Store) UpdatePort(ctx context.Context, id int64, in PortInput) (*Port, error) {
	slugBase := Slugify(in.Game)
	slug, err := s.uniqueSlug(ctx, slugBase, id)
	if err != nil {
		return nil, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx, `
		UPDATE ports SET slug=$1, game=$2, game_sort=$3, version=$4, file_size=$5,
		       genre_slug=$6, description=$7, author=$8, category_slug=$9,
		       status_slug=$10, image_url=$11, download_url=$12, iso_url=$13,
		       source_url=$14, install_notes=$15,
		       published=$16, needs_own_data=$17, updated_at=now()
		WHERE id=$18`,
		slug, in.Game, strings.ToLower(in.Game), in.Version, in.FileSize,
		in.GenreSlug, in.Description, in.Author, in.CategorySlug, in.StatusSlug,
		in.ImageURL, in.DownloadURL, in.IsoURL, in.SourceURL, in.InstallNotes,
		in.Published, in.NeedsOwnData, id)
	if err != nil {
		return nil, fmt.Errorf("update port: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	if err := replacePlatforms(ctx, tx, id, in.Platforms); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getPortByID(ctx, id)
}

func replacePlatforms(ctx context.Context, tx pgx.Tx, portID int64, slugs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM port_platforms WHERE port_id = $1`, portID); err != nil {
		return err
	}
	for _, sl := range slugs {
		if strings.TrimSpace(sl) == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO port_platforms (port_id, platform_slug) VALUES ($1,$2)
			 ON CONFLICT DO NOTHING`, portID, sl); err != nil {
			return fmt.Errorf("platform %q: %w", sl, err)
		}
	}
	return nil
}

// DeletePort menghapus port.
func (s *Store) DeletePort(ctx context.Context, id int64) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM ports WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) getPortByID(ctx context.Context, id int64) (*Port, error) {
	var p Port
	if err := scanPort(s.Pool.QueryRow(ctx,
		"SELECT "+portColumns+" FROM v_port_detail WHERE id = $1", id), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// GetPortByID dipakai admin.
func (s *Store) GetPortByID(ctx context.Context, id int64) (*Port, error) {
	return s.getPortByID(ctx, id)
}

// ---------- Analitik ----------

// CatatEvent menyimpan satu kejadian.
func (s *Store) CatatEvent(ctx context.Context, kind string, portID *int64, query *string) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO events (kind, port_id, query) VALUES ($1,$2,$3)`,
		kind, portID, query)
	return err
}

// TambahView menaikkan penghitung tampilan.
func (s *Store) TambahView(ctx context.Context, id int64) {
	_, _ = s.Pool.Exec(ctx, `UPDATE ports SET view_count = view_count + 1 WHERE id = $1`, id)
}

// TambahDownload menaikkan penghitung unduhan.
func (s *Store) TambahDownload(ctx context.Context, id int64) {
	_, _ = s.Pool.Exec(ctx, `UPDATE ports SET download_count = download_count + 1 WHERE id = $1`, id)
}

// StatAdmin merangkum angka untuk dashboard.
func (s *Store) StatAdmin(ctx context.Context) (*StatAdmin, error) {
	var st StatAdmin
	err := s.Pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM ports),
		  (SELECT count(*) FROM ports WHERE published),
		  (SELECT count(*) FROM ports WHERE NOT published),
		  (SELECT COALESCE(sum(view_count),0) FROM ports),
		  (SELECT COALESCE(sum(download_count),0) FROM ports),
		  (SELECT count(*) FROM events WHERE kind='view'   AND created_at >= date_trunc('day', now())),
		  (SELECT count(*) FROM events WHERE kind='download' AND created_at >= date_trunc('day', now())),
		  (SELECT count(*) FROM events WHERE kind='view'   AND created_at >= now() - interval '7 days'),
		  (SELECT count(*) FROM events WHERE kind='download' AND created_at >= now() - interval '7 days')
	`).Scan(&st.TotalPort, &st.Terbit, &st.Draf, &st.TotalView, &st.TotalDownload,
		&st.ViewHariIni, &st.DownldHariIni, &st.View7Hari, &st.Downld7Hari)
	return &st, err
}

// GrafikHarian mengembalikan seri 14 hari terakhir.
func (s *Store) GrafikHarian(ctx context.Context, hari int) ([]TitikHarian, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH d AS (
		  SELECT generate_series(
		    date_trunc('day', now()) - ($1::int - 1) * interval '1 day',
		    date_trunc('day', now()),
		    interval '1 day') AS t
		)
		SELECT to_char(d.t, 'YYYY-MM-DD'),
		       (SELECT count(*) FROM events e WHERE e.kind='view'
		          AND e.created_at >= d.t AND e.created_at < d.t + interval '1 day'),
		       (SELECT count(*) FROM events e WHERE e.kind='download'
		          AND e.created_at >= d.t AND e.created_at < d.t + interval '1 day')
		FROM d ORDER BY d.t`, hari)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TitikHarian{}
	for rows.Next() {
		var t TitikHarian
		if err := rows.Scan(&t.Tanggal, &t.View, &t.Download); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// PortPopuler mengembalikan port dengan view terbanyak.
func (s *Store) PortPopuler(ctx context.Context, limit int) ([]PortPopuler, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, game, slug, view_count, download_count, image_url
		FROM ports ORDER BY view_count DESC, download_count DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PortPopuler{}
	for rows.Next() {
		var p PortPopuler
		if err := rows.Scan(&p.ID, &p.Game, &p.Slug, &p.ViewCount, &p.DownloadCount, &p.ImageURL); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SitemapEntries mengembalikan slug + waktu ubah seluruh port terbit.
// Dipakai /sitemap.xml agar crawler tahu halaman mana yang perlu dipindai ulang.
func (s *Store) SitemapEntries(ctx context.Context) ([]SitemapEntry, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT slug, updated_at FROM ports
		WHERE published ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var e SitemapEntry
		if err := rows.Scan(&e.Slug, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PencarianTeratas mengembalikan query terbanyak.
func (s *Store) PencarianTeratas(ctx context.Context, limit int) ([]PencarianTeratas, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT query, count(*) FROM events
		WHERE kind='search' AND query IS NOT NULL AND query <> ''
		  AND created_at >= now() - interval '30 days'
		GROUP BY query ORDER BY count(*) DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PencarianTeratas{}
	for rows.Next() {
		var p PencarianTeratas
		if err := rows.Scan(&p.Query, &p.Jumlah); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}