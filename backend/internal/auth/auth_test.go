package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gopod/internal/db"
)

func setupTestHandler(t *testing.T) (*Handler, *SQLiteRepository) {
	tempDB := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	repo := NewSQLiteRepository(database.DB)
	svc := NewService(repo)
	handler := NewHandler(svc)
	return handler, repo
}

func TestAuthStatusAndSetupFlow(t *testing.T) {
	handler, _ := setupTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 1. Initial status: should be uninitialized
	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var statusResp StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&statusResp); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}
	if statusResp.Initialized {
		t.Fatalf("expected uninitialized system, got initialized: true")
	}
	if statusResp.Authenticated {
		t.Fatalf("expected unauthenticated, got authenticated: true")
	}

	// 2. Perform Setup
	setupPayload := SetupRequest{
		Name:     "Test Admin",
		Email:    "admin@test.local",
		Password: "password1234",
	}
	setupBody, _ := json.Marshal(setupPayload)
	req = httptest.NewRequest("POST", "/api/auth/setup", bytes.NewReader(setupBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created for setup, got %d: %s", w.Code, w.Body.String())
	}

	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "gopod_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected gopod_session cookie to be set")
	}

	// 3. Second Setup attempt: MUST return 403 Forbidden!
	req = httptest.NewRequest("POST", "/api/auth/setup", bytes.NewReader(setupBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden for repeated setup, got %d", w.Code)
	}

	// 4. Status with session cookie: should be initialized & authenticated
	req = httptest.NewRequest("GET", "/api/auth/status", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var authStatus StatusResponse
	_ = json.NewDecoder(w.Body).Decode(&authStatus)
	if !authStatus.Initialized || !authStatus.Authenticated {
		t.Fatalf("expected initialized & authenticated, got init=%v auth=%v", authStatus.Initialized, authStatus.Authenticated)
	}
	if authStatus.User == nil || authStatus.User.Email != "admin@test.local" {
		t.Fatalf("expected user email admin@test.local, got %v", authStatus.User)
	}

	// 5. Login with invalid password
	loginFail := LoginRequest{
		Email:    "admin@test.local",
		Password: "wrongpassword",
	}
	failBody, _ := json.Marshal(loginFail)
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(failBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized for bad login, got %d", w.Code)
	}

	// 6. Login with correct password
	loginSuccess := LoginRequest{
		Email:    "admin@test.local",
		Password: "password1234",
	}
	successBody, _ := json.Marshal(loginSuccess)
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(successBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for valid login, got %d", w.Code)
	}

	// 7. Logout
	req = httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for logout, got %d", w.Code)
	}
}
