package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type mockAuthRepo struct {
	countCalls atomic.Int64
	userCount  int
	user       *User
	session    *Session
}

func (m *mockAuthRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return nil, nil
}
func (m *mockAuthRepo) GetUserByID(ctx context.Context, id string) (*User, error) {
	return m.user, nil
}
func (m *mockAuthRepo) CreateUser(ctx context.Context, user User) error {
	return nil
}
func (m *mockAuthRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	return nil
}
func (m *mockAuthRepo) CountUsers(ctx context.Context) (int, error) {
	m.countCalls.Add(1)
	return m.userCount, nil
}
func (m *mockAuthRepo) CreateSession(ctx context.Context, session Session) error {
	return nil
}
func (m *mockAuthRepo) GetSession(ctx context.Context, token string) (*Session, *User, error) {
	if token == "valid-token" && m.user != nil {
		return m.session, m.user, nil
	}
	return nil, nil, nil
}
func (m *mockAuthRepo) DeleteSession(ctx context.Context, token string) error {
	return nil
}
func (m *mockAuthRepo) PruneExpiredSessions(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *mockAuthRepo) DeleteOtherSessions(ctx context.Context, userID, keepToken string) (int64, error) {
	return 0, nil
}

func TestRequireAuthMiddlewareCaching(t *testing.T) {
	repo := &mockAuthRepo{
		userCount: 1,
		user: &User{
			ID:    "u-1",
			Email: "admin@gopod.internal",
		},
		session: &Session{
			Token:     "valid-token",
			UserID:    "u-1",
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}

	authService := NewService(repo)
	mw := NewMiddleware(authService)

	handlerCalled := 0
	handler := mw.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled++
		w.WriteHeader(http.StatusOK)
	})

	// Make 3 requests with valid token
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()
		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected status 200, got %d", i, rr.Code)
		}
	}

	if handlerCalled != 3 {
		t.Errorf("expected handler called 3 times, got %d", handlerCalled)
	}

	// Verify CountUsers was called only once due to atomic caching
	if calls := repo.countCalls.Load(); calls != 1 {
		t.Errorf("expected CountUsers called 1 time (cached), got %d calls", calls)
	}
}

func TestRequireAuthAllowQueryRejectsSessionQueryToken(t *testing.T) {
	repo := &mockAuthRepo{
		userCount: 1,
		user: &User{
			ID:    "u-1",
			Email: "admin@gopod.internal",
		},
		session: &Session{
			Token:     "valid-token",
			UserID:    "u-1",
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}

	authService := NewService(repo)
	mw := NewMiddleware(authService)
	handler := mw.RequireAuthAllowQuery(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Long-lived session token in ?token= query param must be rejected.
	req := httptest.NewRequest("GET", "/api/deployments/dep-1/logs/stream?token=valid-token", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when passing session token in query param, got %d", rr.Code)
	}

	// Single-use stream grant must be accepted once and rejected on replay.
	grant, err := authService.IssueStreamGrant("u-1")
	if err != nil {
		t.Fatalf("IssueStreamGrant: %v", err)
	}
	req2 := httptest.NewRequest("GET", "/api/deployments/dep-1/logs/stream?grant="+grant, nil)
	rr2 := httptest.NewRecorder()
	handler(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid single-use stream grant, got %d", rr2.Code)
	}
	rr3 := httptest.NewRecorder()
	handler(rr3, httptest.NewRequest("GET", "/api/deployments/dep-1/logs/stream?grant="+grant, nil))
	if rr3.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on replayed stream grant, got %d", rr3.Code)
	}
}
