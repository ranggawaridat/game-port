package db

import (
	"context"
	"fmt"
	"strings"
)

// PortFilter adalah parameter pencarian dari query string.
type PortFilter struct {
	Q        string
	Kategori []string
	Status   []string
	Platform []string
	Own      *bool
	Sort     string
	Limit    int
	Offset   int
}

var sortSQL = map[string]string{
	"nama":      "game_sort ASC",
	"nama-desc": "game_sort DESC",
	"kategori":  "category_urutan ASC, game_sort ASC",
	"status":    "status_bobot DESC, game_sort ASC",
}

// buildWhere menyusun klausa WHERE beserta argumennya.
func buildWhere(f PortFilter) (string, []any) {
	var conds []string
	var args []any

	n := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
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

const portColumns = `id, slug, game, game_sort, description, author, author_url,
	repo_url, needs_own_data, category_slug, category_name, category_urutan,
	status_slug, status_name, status_warna, status_bobot,
	platforms::text[], engines::text[], features::text[]`

// ListPorts menjalankan pencarian berhalaman di view.
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
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Game, &p.GameSort, &p.Description, &p.Author,
			&p.AuthorURL, &p.RepoURL, &p.NeedsOwnData, &p.CategorySlug,
			&p.CategoryName, &p.CategoryOrder, &p.StatusSlug, &p.StatusName,
			&p.StatusColor, &p.StatusWeight, &p.Platforms, &p.Engines, &p.Features,
		); err != nil {
			return nil, fmt.Errorf("scan port: %w", err)
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &PortList{Total: total, Limit: f.Limit, Offset: f.Offset, Items: items}, nil
}

// GetPort mengambil satu port berdasarkan slug, beserta 6 port terkait sekategori.
func (s *Store) GetPort(ctx context.Context, slug string) (*PortDetail, error) {
	var p Port
	err := s.Pool.QueryRow(ctx,
		"SELECT "+portColumns+" FROM v_port_detail WHERE slug = $1", slug,
	).Scan(
		&p.ID, &p.Slug, &p.Game, &p.GameSort, &p.Description, &p.Author,
		&p.AuthorURL, &p.RepoURL, &p.NeedsOwnData, &p.CategorySlug,
		&p.CategoryName, &p.CategoryOrder, &p.StatusSlug, &p.StatusName,
		&p.StatusColor, &p.StatusWeight, &p.Platforms, &p.Engines, &p.Features,
	)
	if err != nil {
		return nil, err
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT slug, game FROM v_port_detail
		WHERE category_slug = $1 AND slug <> $2
		ORDER BY game_sort LIMIT 6`, p.CategorySlug, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	related := make([]Ref, 0, 6)
	for rows.Next() {
		var r Ref
		if err := rows.Scan(&r.Slug, &r.Game); err != nil {
			return nil, err
		}
		related = append(related, r)
	}
	return &PortDetail{Port: p, Related: related}, rows.Err()
}

// Statistik mengambil agregat dari view.
func (s *Store) Statistik(ctx context.Context) (*Statistik, error) {
	var st Statistik
	err := s.Pool.QueryRow(ctx, `
		SELECT total_port, total_author, butuh_data_asli,
		       stabil, beta, eksperimental, alpha
		FROM v_statistik`).Scan(
		&st.TotalPort, &st.TotalAuthor, &st.ButuhDataAsli,
		&st.Stabil, &st.Beta, &st.Eksperimental, &st.Alpha,
	)
	return &st, err
}

// KategoriList mengembalikan semua kategori dengan jumlah port.
func (s *Store) KategoriList(ctx context.Context) ([]Kategori, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.slug, c.name, c.urutan, count(p.id)
		FROM categories c LEFT JOIN ports p ON p.category_slug = c.slug
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

// StatusList mengembalikan status beserta jumlah.
func (s *Store) StatusList(ctx context.Context) ([]Status, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT s.slug, s.name, s.warna, s.bobot, count(p.id)
		FROM statuses s LEFT JOIN ports p ON p.status_slug = s.slug
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

// PlatformList mengembalikan platform beserta jumlah.
func (s *Store) PlatformList(ctx context.Context) ([]Platform, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT pl.slug, pl.name, count(*)
		FROM platforms pl JOIN port_platforms pp ON pp.platform_slug = pl.slug
		GROUP BY pl.slug, pl.name ORDER BY count(*) DESC`)
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

// EngineList mengembalikan engine beserta jumlah.
func (s *Store) EngineList(ctx context.Context) ([]Engine, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT e.slug, e.name, count(*)
		FROM engines e JOIN port_engines pe ON pe.engine_slug = e.slug
		GROUP BY e.slug, e.name ORDER BY count(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Engine{}
	for rows.Next() {
		var e Engine
		if err := rows.Scan(&e.Slug, &e.Name, &e.Jumlah); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}