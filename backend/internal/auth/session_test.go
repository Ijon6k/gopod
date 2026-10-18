package auth

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopod/internal/db"
)

func newSessionTestRepo(t *testing.T) (*SQLiteRepository, context.Context) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "sessions.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	repo := NewSQLiteRepository(database.DB)
	ctx := context.Background()
	if err := repo.CreateUser(ctx, User{
		ID:           "u-sess",
		Email:        "sess@test.local",
		PasswordHash: "hash",
		Name:         "Session Tester",
		Role:         "admin",
	}); err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	return repo, ctx
}

func TestCreateSessionStoresHashedToken(t *testing.T) {
	repo, ctx := newSessionTestRepo(t)
	raw := "raw-session-token"
	if err := repo.CreateSession(ctx, Session{
		Token:     raw,
		UserID:    "u-sess",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	var stored string
	if err := repo.db.QueryRowContext(ctx, "SELECT token FROM sessions").Scan(&stored); err != nil {
		t.Fatalf("read stored token: %v", err)
	}
	if stored == raw {
		t.Fatalf("session token stored in plaintext")
	}
	if !strings.HasPrefix(stored, sessionTokenPrefix) {
		t.Fatalf("stored token must carry the %q prefix, got %q", sessionTokenPrefix, stored)
	}
	if stored != hashSessionToken(raw) {
		t.Fatalf("stored token is not the expected hash")
	}

	// The raw token resolves; the stored (hashed) form does not double-hash.
	s, u, err := repo.GetSession(ctx, raw)
	if err != nil || s == nil || u == nil {
		t.Fatalf("raw token must resolve (session=%v user=%v err=%v)", s, u, err)
	}
	s, u, err = repo.GetSession(ctx, stored)
	if err != nil || s != nil || u != nil {
		t.Fatalf("stored hashed token must not be usable as a client token")
	}
}

func TestGetSessionLegacyPlaintextFallback(t *testing.T) {
	repo, ctx := newSessionTestRepo(t)
	if _, err := repo.db.ExecContext(ctx,
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		"legacy-plaintext-token", "u-sess", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	s, u, err := repo.GetSession(ctx, "legacy-plaintext-token")
	if err != nil || s == nil || u == nil {
		t.Fatalf("legacy plaintext token must still resolve (session=%v user=%v err=%v)", s, u, err)
	}
}

func TestMigrateSessionTokensIsIdempotent(t *testing.T) {
	repo, ctx := newSessionTestRepo(t)
	if _, err := repo.db.ExecContext(ctx,
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		"legacy-token-a", "u-sess", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := repo.CreateSession(ctx, Session{
		Token:     "already-hashed",
		UserID:    "u-sess",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create hashed session: %v", err)
	}

	migrated, err := repo.MigrateSessionTokens(ctx)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("expected 1 migration, got %d", migrated)
	}

	migrated, err = repo.MigrateSessionTokens(ctx)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if migrated != 0 {
		t.Fatalf("migration must be idempotent, second run migrated %d", migrated)
	}

	var plaintext int
	if err := repo.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sessions WHERE token NOT LIKE ?", sessionTokenPrefix+"%").Scan(&plaintext); err != nil {
		t.Fatalf("count plaintext rows: %v", err)
	}
	if plaintext != 0 {
		t.Fatalf("expected no plaintext session tokens after migration, found %d", plaintext)
	}

	// Both sessions keep working through their raw tokens.
	if s, _, err := repo.GetSession(ctx, "legacy-token-a"); err != nil || s == nil {
		t.Fatalf("migrated token must resolve (session=%v err=%v)", s, err)
	}
	if s, _, err := repo.GetSession(ctx, "already-hashed"); err != nil || s == nil {
		t.Fatalf("pre-hashed token must resolve (session=%v err=%v)", s, err)
	}
}

func TestPruneExpiredSessions(t *testing.T) {
	repo, ctx := newSessionTestRepo(t)
	if err := repo.CreateSession(ctx, Session{
		Token:     "expired-token",
		UserID:    "u-sess",
		ExpiresAt: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	if err := repo.CreateSession(ctx, Session{
		Token:     "valid-token",
		UserID:    "u-sess",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create valid session: %v", err)
	}

	removed, err := repo.PruneExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 pruned session, got %d", removed)
	}
	if s, _, _ := repo.GetSession(ctx, "expired-token"); s != nil {
		t.Fatalf("expired session must be gone")
	}
	if s, _, err := repo.GetSession(ctx, "valid-token"); err != nil || s == nil {
		t.Fatalf("valid session must survive (session=%v err=%v)", s, err)
	}
}

func TestDeleteSessionRemovesHashedAndLegacyRows(t *testing.T) {
	repo, ctx := newSessionTestRepo(t)

	if err := repo.CreateSession(ctx, Session{
		Token:     "delete-me",
		UserID:    "u-sess",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := repo.DeleteSession(ctx, "delete-me"); err != nil {
		t.Fatalf("delete hashed session: %v", err)
	}
	if s, _, _ := repo.GetSession(ctx, "delete-me"); s != nil {
		t.Fatalf("deleted session must not resolve")
	}

	if _, err := repo.db.ExecContext(ctx,
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		"legacy-delete", "u-sess", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := repo.DeleteSession(ctx, "legacy-delete"); err != nil {
		t.Fatalf("delete legacy session: %v", err)
	}
	if s, _, _ := repo.GetSession(ctx, "legacy-delete"); s != nil {
		t.Fatalf("deleted legacy session must not resolve")
	}
}
