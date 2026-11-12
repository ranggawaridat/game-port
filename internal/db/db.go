// Package db menyediakan koneksi pool PostgreSQL dan model data.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store membungkus connection pool.
type Store struct {
	Pool *pgxpool.Pool
}

// New membuat pool dan memverifikasi koneksi dengan ping.
func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("buka pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{Pool: pool}, nil
}

// Close menutup pool.
func (s *Store) Close() { s.Pool.Close() }

// Health mengembalikan info koneksi untuk /api/health.
func (s *Store) Health(ctx context.Context) (map[string]any, error) {
	var user, database, version string
	err := s.Pool.QueryRow(ctx,
		`SELECT current_user::text, current_database()::text, version()::text`,
	).Scan(&user, &database, &version)
	if err != nil {
		return nil, err
	}

	stats := s.Pool.Stat()
	return map[string]any{
		"ok":            true,
		"user":          user,
		"database":      database,
		"version":       version,
		"pool_total":    stats.TotalConns(),
		"pool_idle":     stats.IdleConns(),
		"pool_acquired": stats.AcquiredConns(),
	}, nil
}