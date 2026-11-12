// Package config memuat konfigurasi dari environment variable.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config adalah seluruh konfigurasi runtime aplikasi.
type Config struct {
	Port         string
	DatabaseURL  string
	CORSOrigins  []string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	StaticDir    string
}

// Load membaca environment. Mengembalikan error bila kredensial wajib kosong.
func Load() (*Config, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		host := envOr("GAMEDB_HOST", "db")
		port := envOr("GAMEDB_PORT", "5432")
		name := envOr("GAMEDB_NAME", "gamedb")
		user := os.Getenv("GAMEDB_USER")
		pass := os.Getenv("GAMEDB_PASSWORD")
		if user == "" {
			return nil, fmt.Errorf("GAMEDB_USER kosong: set di environment atau pakai DATABASE_URL")
		}
		// User dan password WAJIB di-escape: password sering mengandung
		// karakter seperti ':' dan '/' yang merusak URL DSN.
		dsn = fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=disable",
			url.UserPassword(user, pass).String(),
			host, port, name)
	}

	origins := []string{"*"}
	if raw := strings.TrimSpace(os.Getenv("CORS_ORIGINS")); raw != "" {
		origins = nil
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
	}

	return &Config{
		Port:         envOr("APP_PORT", "8181"),
		DatabaseURL:  dsn,
		CORSOrigins:  origins,
		ReadTimeout:  durOr("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout: durOr("HTTP_WRITE_TIMEOUT", 30*time.Second),
		StaticDir:    envOr("STATIC_DIR", "static"),
	}, nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func durOr(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return def
}