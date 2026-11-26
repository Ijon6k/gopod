package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gopod/internal/db"
)

func TestRequireAdminAndProjectIsolation(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "rbac.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	_, _ = database.DB.ExecContext(ctx, `INSERT INTO projects (id, name) VALUES ('proj-a', 'Alpha'), ('proj-b', 'Beta')`)

	authRepo := NewSQLiteRepository(database.DB)
	authSvc := NewService(authRepo)
	adminUser, adminTok, err := authSvc.SetupAdmin(ctx, "Admin", "admin@gopod.local", "adminpass123")
	if err != nil {
		t.Fatalf("SetupAdmin: %v", err)
	}
	_ = adminUser

	ac := NewAccessChecker(database.DB)
	member, err := ac.CreateManagedUser(ctx, CreateUserRequest{
		Name:       "Friend",
		Email:      "friend@gopod.local",
		Password:   "friendpass123",
		Role:       "member",
		ProjectIDs: []string{"proj-a"},
	})
	if err != nil {
		t.Fatalf("CreateManagedUser: %v", err)
	}
	_, memberTok, err := authSvc.Login(ctx, member.Email, "friendpass123")
	if err != nil {
		t.Fatalf("member login: %v", err)
	}

	mw := NewMiddleware(authSvc)
	mw.SetAccessChecker(ac)

	// 1. RequireAdmin denies member with 403 and allows admin with 200
	adminOnly := mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	reqMember := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	reqMember.Header.Set("Authorization", "Bearer "+memberTok)
	rrMember := httptest.NewRecorder()
	adminOnly(rrMember, reqMember)
	if rrMember.Code != http.StatusForbidden {
		t.Fatalf("member on RequireAdmin got %d, want 403", rrMember.Code)
	}

	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminTok)
	rrAdmin := httptest.NewRecorder()
	adminOnly(rrAdmin, reqAdmin)
	if rrAdmin.Code != http.StatusOK {
		t.Fatalf("admin on RequireAdmin got %d, want 200", rrAdmin.Code)
	}

	// 2. Project access isolation checks
	projectGuard := mw.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		pid := r.URL.Query().Get("projectId")
		if !mw.CanAccessProject(r, pid) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	reqProjA := httptest.NewRequest(http.MethodGet, "/api/services?projectId=proj-a", nil)
	reqProjA.Header.Set("Authorization", "Bearer "+memberTok)
	rrA := httptest.NewRecorder()
	projectGuard(rrA, reqProjA)
	if rrA.Code != http.StatusOK {
		t.Fatalf("member on assigned proj-a got %d, want 200", rrA.Code)
	}

	reqProjB := httptest.NewRequest(http.MethodGet, "/api/services?projectId=proj-b", nil)
	reqProjB.Header.Set("Authorization", "Bearer "+memberTok)
	rrB := httptest.NewRecorder()
	projectGuard(rrB, reqProjB)
	if rrB.Code != http.StatusForbidden {
		t.Fatalf("member on unassigned proj-b got %d, want 403", rrB.Code)
	}

	// 3. Guard against deleting last admin or self
	if err := ac.DeleteManagedUser(ctx, adminUser.ID, adminUser.ID); err == nil {
		t.Fatal("expected error deleting own active admin account")
	}
	if err := ac.DeleteManagedUser(ctx, "other-id", adminUser.ID); err == nil {
		t.Fatal("expected error deleting last remaining admin")
	}
	_ = time.Second
}

