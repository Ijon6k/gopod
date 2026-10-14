package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type failedAttempt struct {
	count     int
	firstSeen time.Time
}

// LoginRateLimiter enforces a maximum number of failed authentication attempts per IP within a window.
type LoginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*failedAttempt
	maxFails int
	window   time.Duration
}

// NewLoginRateLimiter creates a new LoginRateLimiter instance.
func NewLoginRateLimiter(maxFails int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		attempts: make(map[string]*failedAttempt),
		maxFails: maxFails,
		window:   window,
	}
}

func (l *LoginRateLimiter) getIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// IsAllowed returns true if the client IP has not exceeded the failure threshold.
func (l *LoginRateLimiter) IsAllowed(r *http.Request) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	ip := l.getIP(r)
	attempt, exists := l.attempts[ip]
	if !exists {
		return true
	}

	if time.Since(attempt.firstSeen) > l.window {
		delete(l.attempts, ip)
		return true
	}

	return attempt.count < l.maxFails
}

// RecordFailure records a failed login attempt for the client IP.
func (l *LoginRateLimiter) RecordFailure(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ip := l.getIP(r)
	attempt, exists := l.attempts[ip]
	if !exists || time.Since(attempt.firstSeen) > l.window {
		l.attempts[ip] = &failedAttempt{
			count:     1,
			firstSeen: time.Now(),
		}
		return
	}

	attempt.count++
}

// Reset clears recorded failed attempts upon successful authentication.
func (l *LoginRateLimiter) Reset(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ip := l.getIP(r)
	delete(l.attempts, ip)
}
