package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---------- Safelink: nonce halaman & token unduhan ----------
//
// Alur: halaman gerbang menerbitkan nonce -> halaman meminta token dengan
// nonce itu -> token dipakai sekali untuk menebus URL penyedia.
// Semua penandaan bersifat atomik lewat UPDATE ... WHERE belum_dipakai,
// jadi dua permintaan bersamaan tidak bisa memakai nonce/token yang sama.

// BuatNonceHalaman menerbitkan nonce untuk satu kunjungan halaman gerbang.
func (s *Store) BuatNonceHalaman(ctx context.Context, nonce string, portID int64, kind, ipHash string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO safelink_pages (nonce, port_id, kind, ip_hash)
		VALUES ($1,$2,$3,$4)`, nonce, portID, kind, ipHash)
	if err != nil {
		return fmt.Errorf("buat nonce: %w", err)
	}
	return nil
}

// ErrBelumWaktu dikembalikan bila halaman gerbang belum melewati masa tunggu.
var ErrBelumWaktu = errors.New("belum melewati masa tunggu")

// PakaiNonceHalaman menandai nonce terpakai dan mengembalikan tujuannya.
//
// Nonce hanya sah bila:
//   - belum pernah dipakai,
//   - umurnya sudah melewati minAge (masa tunggu halaman gerbang), dan
//   - belum melewati maxAge.
//
// Masa tunggu diperiksa di server, bukan dari timer JavaScript, sehingga
// mempercepat timer di browser tidak berpengaruh.
func (s *Store) PakaiNonceHalaman(ctx context.Context, nonce string, minAge, maxAge time.Duration) (int64, string, error) {
	var portID int64
	var kind string
	err := s.Pool.QueryRow(ctx, `
		UPDATE safelink_pages SET used_at = now()
		WHERE nonce = $1
		  AND used_at IS NULL
		  AND created_at <= now() - make_interval(secs => $2)
		  AND created_at >  now() - make_interval(secs => $3)
		RETURNING port_id, kind`,
		nonce, minAge.Seconds(), maxAge.Seconds()).Scan(&portID, &kind)

	if err == nil {
		return portID, kind, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, "", fmt.Errorf("pakai nonce: %w", err)
	}

	// Tidak cocok. Bedakan "belum waktunya" dari "nonce tidak sah" supaya
	// halaman bisa memberi pesan yang benar.
	var dibuat time.Time
	var dipakai *time.Time
	e := s.Pool.QueryRow(ctx,
		`SELECT created_at, used_at FROM safelink_pages WHERE nonce = $1`, nonce).
		Scan(&dibuat, &dipakai)
	if errors.Is(e, pgx.ErrNoRows) || dipakai != nil {
		return 0, "", ErrNotFound
	}
	if e != nil {
		return 0, "", fmt.Errorf("cek nonce: %w", e)
	}
	if time.Since(dibuat) < minAge {
		return 0, "", ErrBelumWaktu
	}
	return 0, "", ErrNotFound
}

// TerbitkanTokenDownload menyimpan hash token sekali pakai.
func (s *Store) TerbitkanTokenDownload(ctx context.Context, hash string, portID int64, kind, ipHash string, ttl time.Duration) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO download_tokens (token_hash, port_id, kind, ip_hash, expires_at)
		VALUES ($1,$2,$3,$4, now() + make_interval(secs => $5))`,
		hash, portID, kind, ipHash, ttl.Seconds())
	if err != nil {
		return fmt.Errorf("terbitkan token: %w", err)
	}
	return nil
}

// PakaiTokenDownload menandai token terpakai dan mengembalikan tujuannya.
// Token yang sudah dipakai atau kedaluwarsa tidak akan pernah cocok.
func (s *Store) PakaiTokenDownload(ctx context.Context, hash string) (int64, string, error) {
	var portID int64
	var kind string
	err := s.Pool.QueryRow(ctx, `
		UPDATE download_tokens SET used_at = now()
		WHERE token_hash = $1
		  AND used_at IS NULL
		  AND expires_at > now()
		RETURNING port_id, kind`, hash).Scan(&portID, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", ErrNotFound
	}
	if err != nil {
		return 0, "", fmt.Errorf("pakai token: %w", err)
	}
	return portID, kind, nil
}

// BersihkanSafelink membuang nonce dan token yang sudah tidak mungkin dipakai.
// Dipanggil berkala supaya tabel tidak tumbuh tanpa batas.
func (s *Store) BersihkanSafelink(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM safelink_pages WHERE created_at < now() - interval '1 day';
		DELETE FROM download_tokens WHERE expires_at < now() - interval '1 day';`)
	if err != nil {
		return fmt.Errorf("bersihkan safelink: %w", err)
	}
	return nil
}
