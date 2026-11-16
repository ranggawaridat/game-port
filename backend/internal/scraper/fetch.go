package scraper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent sengaja menyebut identitas bot dan halaman kontak supaya
// operator situs bisa menghubungi kalau tidak ingin di-scrape.
const DefaultUserAgent = "GamePortBot/0.1 (+https://gameport.web.id/about; admin@gameport.web.id)"

// maxBody membatasi ukuran respons supaya satu halaman besar tidak menghabiskan
// memori. Semua sumber yang didukung jauh di bawah batas ini.
const maxBody = 8 << 20

// ErrDisallowed dikembalikan bila robots.txt situs melarang path tersebut.
var ErrDisallowed = errors.New("dilarang oleh robots.txt")

// Fetcher mengambil halaman dengan sopan: menghormati robots.txt, memberi jeda
// antar permintaan ke host yang sama, dan memakai User-Agent yang jelas.
type Fetcher struct {
	client    *http.Client
	userAgent string
	delay     time.Duration
	log       *slog.Logger

	mu     sync.Mutex
	robots map[string]*robotsEntry
	last   map[string]time.Time
}

type robotsEntry struct {
	rules *robotsFile
	err   error
	// missing=true berarti robots.txt tidak ada (4xx) sehingga semua path boleh.
	missing bool
}

// NewFetcher membuat Fetcher. delay adalah jeda minimum antar permintaan ke
// host yang sama; nilai dari robots.txt yang lebih besar akan dipakai.
func NewFetcher(log *slog.Logger, userAgent string, delay time.Duration) *Fetcher {
	if strings.TrimSpace(userAgent) == "" {
		userAgent = DefaultUserAgent
	}
	if delay <= 0 {
		delay = 1500 * time.Millisecond
	}
	if log == nil {
		log = slog.Default()
	}
	return &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("terlalu banyak redirect")
				}
				return nil
			},
		},
		userAgent: userAgent,
		delay:     delay,
		log:       log,
		robots:    map[string]*robotsEntry{},
		last:      map[string]time.Time{},
	}
}

// Get mengambil body sebuah URL setelah memeriksa robots.txt dan memberi jeda.
func (f *Fetcher) Get(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("URL tidak valid %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("skema URL tidak didukung: %q", u.Scheme)
	}

	entry := f.robotsFor(ctx, u)
	if entry.err != nil {
		return nil, entry.err
	}
	if !entry.missing && !entry.rules.allowed(u.RequestURI(), f.userAgent) {
		return nil, fmt.Errorf("%w: %s", ErrDisallowed, rawURL)
	}

	if err := f.wait(ctx, u.Host, entry); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/html;q=0.9, text/markdown;q=0.9, */*;q=0.5")
	req.Header.Set("Accept-Language", "en")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return nil, fmt.Errorf("GET %s: HTTP %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("baca %s: %w", rawURL, err)
	}
	f.mark(u.Host)
	return body, nil
}

// robotsFor mengambil (dan meng-cache) robots.txt untuk host URL.
func (f *Fetcher) robotsFor(ctx context.Context, u *url.URL) *robotsEntry {
	host := u.Host
	f.mu.Lock()
	if e, ok := f.robots[host]; ok {
		f.mu.Unlock()
		return e
	}
	f.mu.Unlock()

	entry := f.fetchRobots(ctx, u)
	f.mu.Lock()
	f.robots[host] = entry
	f.mu.Unlock()
	return entry
}

func (f *Fetcher) fetchRobots(ctx context.Context, u *url.URL) *robotsEntry {
	raw := u.Scheme + "://" + u.Host + "/robots.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return &robotsEntry{err: err}
	}
	req.Header.Set("User-Agent", f.userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &robotsEntry{err: fmt.Errorf("ambil robots.txt %s: %w", raw, err)}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone ||
		resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		// Tidak ada robots.txt: tidak ada larangan yang dipublikasikan.
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		f.log.Debug("robots.txt tidak tersedia", "host", u.Host, "status", resp.StatusCode)
		return &robotsEntry{missing: true}
	case resp.StatusCode != http.StatusOK:
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		// Jangan menebak: kalau robots.txt tidak bisa dibaca, sumber dilewati.
		return &robotsEntry{err: fmt.Errorf("robots.txt %s: HTTP %d", u.Host, resp.StatusCode)}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return &robotsEntry{err: fmt.Errorf("baca robots.txt %s: %w", u.Host, err)}
	}
	return &robotsEntry{rules: parseRobots(string(body))}
}

// wait memberi jeda supaya tidak menembak satu host berturut-turut.
func (f *Fetcher) wait(ctx context.Context, host string, entry *robotsEntry) error {
	d := f.delay
	if !entry.missing && entry.rules != nil {
		if cd := entry.rules.crawlDelay(f.userAgent); cd > d {
			d = cd
		}
	}

	f.mu.Lock()
	next := f.last[host].Add(d)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	f.last[host] = next
	f.mu.Unlock()

	t := time.NewTimer(time.Until(next))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (f *Fetcher) mark(host string) {
	f.mu.Lock()
	// Tetap pakai jadwal yang sudah dihitung di wait(); ini hanya memastikan
	// host tercatat kalau belum pernah lewat wait (mis. robots.txt 4xx).
	if _, ok := f.last[host]; !ok {
		f.last[host] = time.Now()
	}
	f.mu.Unlock()
}
