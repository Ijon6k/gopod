package auth

import (
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type failedAttempt struct {
	count     int
	firstSeen time.Time
	lastSeen  time.Time
}

// maxTrackedClients caps how many source addresses stay in memory. A
// distributed credential-stuffing run supplies unbounded distinct IPs; once
// the cap is reached expired entries are dropped first, then the oldest live
// ones.
const maxTrackedClients = 4096

// LoginRateLimiter enforces a maximum number of failed authentication attempts per client IP within a window.
type LoginRateLimiter struct {
	mu         sync.Mutex
	attempts   map[string]*failedAttempt
	maxFails   int
	window     time.Duration
	lastPrune  time.Time
	pruneEvery time.Duration
	maxEntries int
}

// NewLoginRateLimiter creates a new LoginRateLimiter instance.
func NewLoginRateLimiter(maxFails int, window time.Duration) *LoginRateLimiter {
	pruneEvery := time.Minute
	if window < pruneEvery {
		pruneEvery = window
	}
	if pruneEvery < 100*time.Millisecond {
		pruneEvery = 100 * time.Millisecond
	}
	return &LoginRateLimiter{
		attempts:   make(map[string]*failedAttempt),
		maxFails:   maxFails,
		window:     window,
		pruneEvery: pruneEvery,
		maxEntries: maxTrackedClients,
	}
}

// getIP returns the client address used as the limiter key.
//
// Forwarded headers are attacker-controlled, so X-Forwarded-For is only
// honored when the direct peer is a trusted proxy (loopback or an entry in
// TRUSTED_PROXIES). In that case the rightmost address that is not itself a
// trusted proxy is used — the first entry is trivially spoofable.
func (l *LoginRateLimiter) getIP(r *http.Request) string {
	peer := remoteHost(r.RemoteAddr)
	if !isTrustedProxy(peer) {
		return peer
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return peer
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		if candidate == "" {
			continue
		}
		if !isTrustedProxy(candidate) {
			return candidate
		}
	}
	return peer
}

// remoteHost extracts the host part of a RemoteAddr ("ip:port").
func remoteHost(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// isTrustedProxy reports whether addr (an IP literal) may supply forwarded
// headers: loopback peers and entries from TRUSTED_PROXIES (comma separated
// IPs or CIDR ranges).
func isTrustedProxy(addr string) bool {
	ip := net.ParseIP(strings.Trim(addr, "[]"))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	for _, entry := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, cidr, err := net.ParseCIDR(entry); err == nil {
			if cidr.Contains(ip) {
				return true
			}
			continue
		}
		if other := net.ParseIP(entry); other != nil && other.Equal(ip) {
			return true
		}
	}
	return false
}

// pruneLocked drops entries whose window has elapsed, then evicts the oldest
// entries while the map exceeds its cap. Callers hold l.mu.
func (l *LoginRateLimiter) pruneLocked(now time.Time) {
	for key, att := range l.attempts {
		if now.Sub(att.lastSeen) > l.window {
			delete(l.attempts, key)
		}
	}
	if l.maxEntries <= 0 || len(l.attempts) <= l.maxEntries {
		return
	}
	type entry struct {
		key      string
		lastSeen time.Time
	}
	entries := make([]entry, 0, len(l.attempts))
	for key, att := range l.attempts {
		entries = append(entries, entry{key: key, lastSeen: att.lastSeen})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].lastSeen.Before(entries[j].lastSeen) })
	for i := 0; i < len(entries)-l.maxEntries; i++ {
		delete(l.attempts, entries[i].key)
	}
}

// maybePruneLocked runs the full-map sweep at most once per prune interval, or
// immediately when the map exceeds its cap. Per-key checks in IsAllowed and
// RecordFailure treat elapsed windows as expired in between, so correctness
// never depends on the sweep cadence.
func (l *LoginRateLimiter) maybePruneLocked(now time.Time) {
	if len(l.attempts) > l.maxEntries || now.Sub(l.lastPrune) >= l.pruneEvery {
		l.pruneLocked(now)
		l.lastPrune = now
	}
}

// IsAllowed returns true if the client IP is below the failure threshold.
func (l *LoginRateLimiter) IsAllowed(r *http.Request) bool {
	ip := l.getIP(r)
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.maybePruneLocked(now)
	att, ok := l.attempts[ip]
	if !ok {
		return true
	}
	if now.Sub(att.firstSeen) > l.window {
		delete(l.attempts, ip)
		return true
	}
	return att.count < l.maxFails
}

// RecordFailure registers a failed login attempt for the client IP.
func (l *LoginRateLimiter) RecordFailure(r *http.Request) {
	ip := l.getIP(r)
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.maybePruneLocked(now)
	att, ok := l.attempts[ip]
	if !ok || now.Sub(att.firstSeen) > l.window {
		l.attempts[ip] = &failedAttempt{count: 1, firstSeen: now, lastSeen: now}
		return
	}
	att.count++
	att.lastSeen = now
}

// Reset clears failures for the client IP (successful login).
func (l *LoginRateLimiter) Reset(r *http.Request) {
	ip := l.getIP(r)

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// prune drops all entries whose window elapsed. The limiter prunes lazily
// (amortized) on calls; the manual entry point exists for tests.
func (l *LoginRateLimiter) prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(time.Now())
	l.lastPrune = time.Now()
}
