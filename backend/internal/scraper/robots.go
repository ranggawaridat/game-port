package scraper

import (
	"strconv"
	"strings"
	"time"
)

// robotsFile adalah subset robots.txt (RFC 9309) yang dipakai bot ini:
// grup User-agent, Allow, Disallow, dan Crawl-delay. Direktif lain diabaikan.
type robotsFile struct {
	groups []robotsGroup
}

type robotsGroup struct {
	agents []string
	allow  []string
	deny   []string
	delay  time.Duration
}

// parseRobots membaca teks robots.txt. Parser sengaja longgar: baris yang
// tidak dikenal dilewati supaya situs dengan ekstensi non-standar tetap bisa
// dibaca, dan aturan yang tidak lengkap tidak membuat bot berhenti.
func parseRobots(text string) *robotsFile {
	f := &robotsFile{}
	var cur *robotsGroup
	lastWasAgent := false

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		i := strings.IndexByte(line, ':')
		if i < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:i]))
		val := strings.TrimSpace(line[i+1:])

		switch key {
		case "user-agent":
			if cur == nil || !lastWasAgent {
				f.groups = append(f.groups, robotsGroup{})
				cur = &f.groups[len(f.groups)-1]
			}
			cur.agents = append(cur.agents, strings.ToLower(val))
			lastWasAgent = true
		case "allow", "disallow":
			lastWasAgent = false
			if cur == nil || val == "" {
				// Disallow kosong berarti "boleh semua".
				continue
			}
			if key == "allow" {
				cur.allow = append(cur.allow, val)
			} else {
				cur.deny = append(cur.deny, val)
			}
		case "crawl-delay":
			lastWasAgent = false
			if cur == nil {
				continue
			}
			if n, err := strconv.ParseFloat(val, 64); err == nil && n > 0 {
				d := time.Duration(n * float64(time.Second))
				if d > cur.delay {
					cur.delay = d
				}
			}
		default:
			lastWasAgent = false
		}
	}
	return f
}

// group memilih grup yang paling spesifik untuk user-agent kita. Nama agen
// dicocokkan sebagai substring (mis. "gameportbot" cocok dengan
// "GamePortBot/0.1"), lalu jatuh ke grup "*".
func (f *robotsFile) group(ua string) *robotsGroup {
	ua = strings.ToLower(ua)
	var best *robotsGroup
	bestLen := -1
	var fallback *robotsGroup

	for i := range f.groups {
		g := &f.groups[i]
		for _, a := range g.agents {
			switch {
			case a == "*":
				if fallback == nil {
					fallback = g
				}
			case a != "" && strings.Contains(ua, a) && len(a) > bestLen:
				best = g
				bestLen = len(a)
			}
		}
	}
	if best != nil {
		return best
	}
	return fallback
}

// allowed menentukan apakah path boleh diambil. Aturan terpanjang menang;
// bila panjangnya sama, Allow menang atas Disallow.
func (f *robotsFile) allowed(path, ua string) bool {
	g := f.group(ua)
	if g == nil {
		return true
	}
	allowLen, denyLen := -1, -1
	for _, p := range g.allow {
		if matchRobots(p, path) && len(p) > allowLen {
			allowLen = len(p)
		}
	}
	for _, p := range g.deny {
		if matchRobots(p, path) && len(p) > denyLen {
			denyLen = len(p)
		}
	}
	if denyLen < 0 {
		return true
	}
	return allowLen >= denyLen
}

func (f *robotsFile) crawlDelay(ua string) time.Duration {
	if g := f.group(ua); g != nil {
		return g.delay
	}
	return 0
}

// matchRobots mencocokkan pola robots.txt dengan path. Pola tanpa "$"
// berlaku sebagai awalan, "*" cocok dengan rangkaian karakter apa pun.
func matchRobots(pattern, path string) bool {
	if strings.HasSuffix(pattern, "$") {
		return globMatch(strings.TrimSuffix(pattern, "$"), path)
	}
	return globMatch(pattern+"*", path)
}

func globMatch(pattern, s string) bool {
	pi, si := 0, 0
	star, mark := -1, 0
	for si < len(s) {
		if pi < len(pattern) && pattern[pi] == s[si] {
			pi++
			si++
			continue
		}
		if pi < len(pattern) && pattern[pi] == '*' {
			star = pi
			mark = si
			pi++
			continue
		}
		if star >= 0 {
			pi = star + 1
			mark++
			si = mark
			continue
		}
		return false
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}
