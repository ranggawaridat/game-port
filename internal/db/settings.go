package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ---------- Pengaturan key/value ----------

// ErrSettingKosong dikembalikan bila key tidak ada di tabel settings.
var ErrSettingKosong = errors.New("setting tidak ada")

// AmbilSetting membaca nilai pengaturan. Bila key tidak ada, kembalikan
// string kosong tanpa error — pemanggil tidak perlu membedakan.
func (s *Store) AmbilSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.Pool.QueryRow(ctx,
		`SELECT value FROM settings WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("ambil setting %s: %w", key, err)
	}
	return v, nil
}

// SimpanSetting menulis atau memperbarui satu pengaturan.
func (s *Store) SimpanSetting(ctx context.Context, key, value string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, value)
	if err != nil {
		return fmt.Errorf("simpan setting %s: %w", key, err)
	}
	return nil
}
