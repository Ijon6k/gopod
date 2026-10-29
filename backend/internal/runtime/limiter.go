package runtime

import (
	"net/http"
	"sync"
	"time"

	"gopod/internal/auth"
)

// ipWindowLimiter is a fixed-window per-IP counter for cheap, sensitive
// endpoints (terminal ticket issuance). It does not distinguish failures
// from successes: any request within an exhausted window is refused. The
// key resolution reuses auth.ClientIP so forwarded headers are only trusted
// from trusted proxies — the same posture as the login limiter.
type ipWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]*windowEntry
	limit   int
	window  time.Duration
}

type windowEntry struct {
	count  int
	resets time.Time
}

func newIPWindowLimiter(limit int, window time.Duration) *ipWindowLimiter {
	return &ipWindowLimiter{
		entries: make(map[string]*windowEntry),
		limit:   limit,
		window:  window,
	}
}

// allow reports whether the request's client may proceed now. Blocked
// entries keep returning false until their window expires, which bounds the
// table by active attackers, not by total traffic.
func (l *ipWindowLimiter) allow(r *http.Request) bool {
	now := time.Now()
	key := auth.ClientIP(r)
	if key == "" {
		key = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if e, ok := l.entries[key]; ok {
		if now.Before(e.resets) {
			if e.count >= l.limit {
				return false
			}
			e.count++
			return true
		}
		// Window expired: start a fresh one.
		delete(l.entries, key)
	}

	l.entries[key] = &windowEntry{count: 1, resets: now.Add(l.window)}

	// Bound memory: evict anything stale beyond a modest watermark so a
	// sparse scan of spoofed sources cannot grow the map without limit.
	if len(l.entries) > 4096 {
		for k, e := range l.entries {
			if now.After(e.resets) {
				delete(l.entries, k)
			}
		}
		if len(l.entries) > 4096 {
			l.entries = make(map[string]*windowEntry)
		}
	}
	return true
}
