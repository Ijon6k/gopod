package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newLimitedRequest(remoteAddr, xff string) *http.Request {
	req := httptest.NewRequest("POST", "/api/auth/login", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return req
}

func TestRateLimiterIgnoresSpoofedXFFFromUntrustedPeer(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	limiter := NewLoginRateLimiter(2, time.Minute)

	// Same peer, rotating spoofed XFF values: the key stays the peer address.
	for i := 0; i < 2; i++ {
		limiter.RecordFailure(newLimitedRequest("9.9.9.9:1234", "10.0.0."+string(rune('1'+i))))
	}
	if limiter.IsAllowed(newLimitedRequest("9.9.9.9:1234", "10.0.0.200")) {
		t.Fatalf("rotating spoofed X-Forwarded-For must not bypass the limiter")
	}
	// A different peer has its own bucket.
	if !limiter.IsAllowed(newLimitedRequest("8.8.8.8:1234", "")) {
		t.Fatalf("unrelated peer must not be limited")
	}
}

func TestRateLimiterUsesRightmostUntrustedXFFFromTrustedProxy(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	limiter := NewLoginRateLimiter(5, time.Minute)

	// Loopback peers are trusted proxies; an attacker-supplied leftmost entry
	// must be ignored in favor of the rightmost (closest) hop.
	limiter.RecordFailure(newLimitedRequest("127.0.0.1:5555", "6.6.6.6, 7.7.7.7"))

	limiter.mu.Lock()
	_, hasReal := limiter.attempts["7.7.7.7"]
	_, hasSpoof := limiter.attempts["6.6.6.6"]
	limiter.mu.Unlock()
	if !hasReal {
		t.Fatalf("expected the rightmost XFF hop to key the limiter")
	}
	if hasSpoof {
		t.Fatalf("leftmost XFF entry must not be used")
	}
}

func TestRateLimiterHonorsTrustedProxiesEnv(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8")
	limiter := NewLoginRateLimiter(5, time.Minute)

	limiter.RecordFailure(newLimitedRequest("10.1.2.3:9999", "5.5.5.5"))

	limiter.mu.Lock()
	_, hasClient := limiter.attempts["5.5.5.5"]
	limiter.mu.Unlock()
	if !hasClient {
		t.Fatalf("XFF from a CIDR-trusted proxy must be honored")
	}
}

func TestRateLimiterFallsBackToPeerWhenAllHopsTrusted(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	limiter := NewLoginRateLimiter(5, time.Minute)

	limiter.RecordFailure(newLimitedRequest("127.0.0.1:5555", "127.0.0.1"))

	limiter.mu.Lock()
	_, hasPeer := limiter.attempts["127.0.0.1"]
	limiter.mu.Unlock()
	if !hasPeer {
		t.Fatalf("expected fallback to the peer address when every XFF hop is trusted")
	}
}

func TestRateLimiterResetAndPrune(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	limiter := NewLoginRateLimiter(1, 40*time.Millisecond)

	limiter.RecordFailure(newLimitedRequest("9.9.9.9:1", ""))
	if limiter.IsAllowed(newLimitedRequest("9.9.9.9:1", "")) {
		t.Fatalf("expected the limiter to block after max failures")
	}

	limiter.Reset(newLimitedRequest("9.9.9.9:1", ""))
	if !limiter.IsAllowed(newLimitedRequest("9.9.9.9:1", "")) {
		t.Fatalf("reset must clear the failures")
	}

	limiter.RecordFailure(newLimitedRequest("9.9.9.9:1", ""))
	time.Sleep(60 * time.Millisecond)
	limiter.prune()
	limiter.mu.Lock()
	remaining := len(limiter.attempts)
	limiter.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected prune to drop elapsed entries, %d left", remaining)
	}
}

func TestRateLimiterEvictsOldestAtCapacity(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	limiter := NewLoginRateLimiter(5, time.Minute)
	limiter.maxEntries = 2

	limiter.RecordFailure(newLimitedRequest("1.1.1.1:1", ""))
	limiter.RecordFailure(newLimitedRequest("2.2.2.2:1", ""))
	limiter.RecordFailure(newLimitedRequest("3.3.3.3:1", ""))

	// Give the entries distinct ages so the eviction order is deterministic.
	now := time.Now()
	limiter.mu.Lock()
	limiter.attempts["1.1.1.1"].lastSeen = now.Add(-3 * time.Second)
	limiter.attempts["2.2.2.2"].lastSeen = now.Add(-2 * time.Second)
	limiter.attempts["3.3.3.3"].lastSeen = now.Add(-1 * time.Second)
	limiter.mu.Unlock()

	// The next check sees len > cap and sweeps: expired first, then oldest.
	limiter.IsAllowed(newLimitedRequest("4.4.4.4:1", ""))

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.attempts) != 2 {
		t.Fatalf("expected the map to stay at the cap, got %d entries", len(limiter.attempts))
	}
	if _, ok := limiter.attempts["1.1.1.1"]; ok {
		t.Fatalf("expected the oldest entry to be evicted first")
	}
	if _, ok := limiter.attempts["3.3.3.3"]; !ok {
		t.Fatalf("expected the newest entries to survive eviction")
	}
}

func TestRateLimiterPruneIsAmortized(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	limiter := NewLoginRateLimiter(1, 20*time.Millisecond)
	// Simulate a long-lived limiter: the sweep interval has not elapsed again.
	limiter.pruneEvery = time.Hour

	limiter.RecordFailure(newLimitedRequest("9.9.9.9:1", ""))
	time.Sleep(40 * time.Millisecond)

	// The stale entry is treated as expired per key even though the full-map
	// sweep has not run yet.
	if !limiter.IsAllowed(newLimitedRequest("9.9.9.9:1", "")) {
		t.Fatalf("elapsed window must allow the client again without a sweep")
	}
	limiter.mu.Lock()
	stale := len(limiter.attempts)
	limiter.mu.Unlock()
	if stale != 0 {
		t.Fatalf("expected the expired entry to be dropped on access, %d left", stale)
	}

	// A fresh failure stays parked until an explicit or timed sweep.
	limiter.RecordFailure(newLimitedRequest("9.9.9.9:1", ""))
	limiter.IsAllowed(newLimitedRequest("8.8.8.8:1", ""))
	limiter.mu.Lock()
	parked := len(limiter.attempts)
	limiter.mu.Unlock()
	if parked != 1 {
		t.Fatalf("expected the sweep to be skipped between intervals, %d entries", parked)
	}
	time.Sleep(40 * time.Millisecond)
	limiter.prune()
	limiter.mu.Lock()
	afterPrune := len(limiter.attempts)
	limiter.mu.Unlock()
	if afterPrune != 0 {
		t.Fatalf("explicit prune must sweep everything elapsed, %d left", afterPrune)
	}
}
