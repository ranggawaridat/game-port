// Package config memuat konfigurasi dari environment variable.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config adalah seluruh konfigurasi runtime.
type Config struct {
	Host         string
	Port         string
	DatabaseURL  string
	CORSOrigins  []string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	StaticDir    string
	UploadDir    string
	// PublicURL adalah origin situs publik (Vercel). Dipakai untuk URL absolut
	// di sitemap.xml dan metadata SEO lain.
	PublicURL string

	// Admin
	AdminPath         string // path rahasia, mis. /panel-x7k2
	AdminPasswordHash string // sha256 hex dari password
	AdminSessionKey   string

	// SafeLink — gerbang iklan sebelum unduhan
	SafeLinkEnabled bool
	SafeLinkWait    int // detik tunggu di halaman gerbang
	SafeLinkTTL     int // umur token, detik
}

// HashPassword menghasilkan sha256 hex. Dipakai juga oleh tool CLI.
func HashPassword(p string) string {
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:])
}

// Load membaca environment dan memvalidasi yang wajib.
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
		// Kredensial WAJIB di-escape: password sering mengandung ':' dan '/'.
		dsn = fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=disable",
			url.UserPassword(user, pass).String(), host, port, name)
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

	adminPath := envOr("ADMIN_PATH", "/panel-x7k2")
	if !strings.HasPrefix(adminPath, "/") {
		adminPath = "/" + adminPath
	}
	adminPath = strings.TrimRight(adminPath, "/")

	// Password admin: terima hash langsung atau plaintext (di-hash di sini).
	pwHash := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD_HASH"))
	if pwHash == "" {
		if pw := os.Getenv("ADMIN_PASSWORD"); pw != "" {
			pwHash = HashPassword(pw)
		}
	}
	if pwHash == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD atau ADMIN_PASSWORD_HASH wajib diisi")
	}

	return &Config{
		Host:              envOr("APP_HOST", "0.0.0.0"),
		Port:              envOr("APP_PORT", "8181"),
		DatabaseURL:       dsn,
		CORSOrigins:       origins,
		ReadTimeout:       durOr("HTTP_READ_TIMEOUT", 20*time.Second),
		WriteTimeout:      durOr("HTTP_WRITE_TIMEOUT", 60*time.Second),
		StaticDir:         envOr("STATIC_DIR", "static"),
		UploadDir:         envOr("UPLOAD_DIR", "uploads"),
		PublicURL:         strings.TrimRight(envOr("PUBLIC_URL", "https://gameport.web.id"), "/"),
		AdminPath:         adminPath,
		AdminPasswordHash: pwHash,
		AdminSessionKey:   envOr("ADMIN_SESSION_KEY", ""),

		SafeLinkEnabled: envOr("SAFELINK_ENABLED", "false") == "true",
		SafeLinkWait:    intOr("SAFELINK_WAIT", 10),
		SafeLinkTTL:     intOr("SAFELINK_TTL", 300),
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

// intOr membaca bilangan bulat positif dari environment, atau memakai default.
func intOr(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}