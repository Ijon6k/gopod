package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestReq(remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/containers/test-ctr/terminal-ticket", nil)
	req.RemoteAddr = remoteAddr
	return req
}

func TestIPWindowLimiter(t *testing.T) {
	// 3 requests allowed per 50ms window.
	limiter := newIPWindowLimiter(3, 50*time.Millisecond)

	reqA := newTestReq("1.2.3.4:1234")
	reqB := newTestReq("5.6.7.8:1234")

	// First 3 requests from IP A are allowed.
	for i := 0; i < 3; i++ {
		if !limiter.allow(reqA) {
			t.Fatalf("request %d from IP A should be allowed", i+1)
		}
	}

	// 4th request from IP A must be blocked.
	if limiter.allow(reqA) {
		t.Fatalf("4th request from IP A must be blocked")
	}

	// IP B has its own budget: 3 requests allowed.
	for i := 0; i < 3; i++ {
		if !limiter.allow(reqB) {
			t.Fatalf("request %d from IP B should be allowed", i+1)
		}
	}
	if limiter.allow(reqB) {
		t.Fatalf("4th request from IP B must be blocked")
	}

	// Wait for window to expire.
	time.Sleep(60 * time.Millisecond)

	// After window expiration, IP A should be allowed again.
	if !limiter.allow(reqA) {
		t.Fatalf("request from IP A after window expiration must be allowed")
	}
}
