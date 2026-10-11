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
func (m *mockAuthRepo) CreateUser(ctx context.Context, user User) error {
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
