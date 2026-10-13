package testutil

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gopod/internal/auth"
	"gopod/internal/db"
)

// NewTestDB creates a temporary SQLite database configured in WAL mode for isolated testing.
// It registers a cleanup handler on t to automatically close the database when the test finishes.
func NewTestDB(t *testing.T) *db.DB {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "test_"+time.Now().Format("150405.000000")+".db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("testutil: failed to initialize test database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	return database
}

// NewTestAuth provisions a test administrator user and active session in database.
// It returns the valid session token and user entity.
func NewTestAuth(t *testing.T, database *db.DB) (string, *auth.User) {
	t.Helper()
	repo := auth.NewSQLiteRepository(database.DB)
	svc := auth.NewService(repo)

	user, token, err := svc.SetupAdmin(
		context.Background(),
		"Test Admin",
		"admin@gopod.test",
		"AdminPass123!@#",
	)
	if err != nil {
		t.Fatalf("testutil: failed to provision test admin: %v", err)
	}
	return token, user
}

// NewAuthRequest constructs an *http.Request pre-populated with authorization headers and session cookies.
func NewAuthRequest(method, target string, body io.Reader, token string) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.AddCookie(&http.Cookie{
			Name:     "gopod_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
	return req
}

// SampleComposeYAML returns a valid multi-container Docker Compose fixture for testing.
func SampleComposeYAML() string {
	return `version: '3.8'
services:
  web:
    image: docker.io/library/nginx:alpine
    ports:
      - "80:80"
    environment:
      PORT: "80"
  redis:
    image: docker.io/library/redis:7-alpine
`
}

// SampleQuadletConfig returns a valid Podman systemd Quadlet container definition fixture.
func SampleQuadletConfig(name string) string {
	return `[Unit]
Description=GOPOD managed container - ` + name + `
After=network-online.target

[Container]
Image=docker.io/library/nginx:alpine
PublishPort=8080:80
AutoUpdate=registry

[Service]
Restart=always

[Install]
WantedBy=default.target
`
}
