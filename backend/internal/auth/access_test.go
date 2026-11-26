package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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
	_, _ = database.DB.ExecContext(ctx, `
		INSERT INTO services (id, project_id, name, type) VALUES
			('svc-a1', 'proj-a', 'api', 'container'),
			('svc-a2', 'proj-a', 'worker', 'container'),
			('svc-b1', 'proj-b', 'billing', 'container')
	`)

	authRepo := NewSQLiteRepository(database.DB)
	authSvc := NewService(authRepo)
	adminUser, adminTok, err := authSvc.SetupAdminWithMeta(ctx, "Admin", "admin@gopod.local", "adminpass123", "10.0.0.1", "Mozilla/5.0 Chrome/130.0")
	if err != nil {
		t.Fatalf("SetupAdmin: %v", err)
	}

	ac := NewAccessChecker(database.DB)
	member, err := ac.CreateManagedUser(ctx, CreateUserRequest{
		Name:     "Friend",
		Email:    "friend@gopod.local",
		Password: "friendpass123",
		Role:     "member",
		Permissions: &UserPermissions{
			CanCreateServices: true,
			CanDeployServices: true,
			CanViewMonitoring: true,
		},
		ProjectScopes: []ProjectScope{
			{ProjectID: "proj-a", ServiceIDs: []string{"svc-a1"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateManagedUser: %v", err)
	}

	// Non-existent project must be rejected strictly.
	if _, err := ac.CreateManagedUser(ctx, CreateUserRequest{
		Name:       "BadScope",
		Email:      "bad@gopod.local",
		Password:   "password123",
		Role:       "member",
		ProjectIDs: []string{"proj-does-not-exist"},
	}); err == nil {
		t.Fatal("expected error assigning unknown project ID")
	}

	_, memberTok1, err := authSvc.LoginWithMeta(ctx, member.Email, "friendpass123", "192.168.1.10", "Mozilla/5.0 Firefox/131.0")
	if err != nil {
		t.Fatalf("member login 1: %v", err)
	}
	_, memberTok2, err := authSvc.LoginWithMeta(ctx, member.Email, "friendpass123", "192.168.1.20", "curl/8.5.0")
	if err != nil {
		t.Fatalf("member login 2: %v", err)
	}

	mw := NewMiddleware(authSvc)
	mw.SetAccessChecker(ac)

	// 1. Fail-closed checks when request has no user context attached
	unauthedReq := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	if mw.CanAccessProject(unauthedReq, "proj-a") {
		t.Fatal("CanAccessProject must fail closed when no user exists in context")
	}
	if mw.CanAccessService(unauthedReq, "proj-a", "svc-a1") {
		t.Fatal("CanAccessService must fail closed when no user exists in context")
	}
	if mw.HasPermission(unauthedReq, PermissionDeployServices) {
		t.Fatal("HasPermission must fail closed when no user exists in context")
	}
	if restricted, allowed := mw.AllowedProjectSet(unauthedReq); !restricted || len(allowed) != 0 {
		t.Fatal("AllowedProjectSet must fail closed when no user exists in context")
	}

	// 2. RequireAdmin & RequirePermission checks
	adminOnly := mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	reqMember := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	reqMember.Header.Set("Authorization", "Bearer "+memberTok1)
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

	revealGuard := mw.RequirePermission(PermissionRevealSecrets, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	reqReveal := httptest.NewRequest(http.MethodPost, "/api/services/svc-a1/env/reveal", nil)
	reqReveal.Header.Set("Authorization", "Bearer "+memberTok1)
	rrReveal := httptest.NewRecorder()
	revealGuard(rrReveal, reqReveal)
	if rrReveal.Code != http.StatusForbidden {
		t.Fatalf("member without canRevealSecrets got %d, want 403", rrReveal.Code)
	}

	// 3. Project AND Service scoping checks
	svcGuard := mw.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		pid := r.URL.Query().Get("projectId")
		sid := r.URL.Query().Get("serviceId")
		if !mw.CanAccessService(r, pid, sid) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range []struct {
		pid, sid string
		want     int
	}{
		{"proj-a", "svc-a1", http.StatusOK},
		{"proj-a", "svc-a2", http.StatusForbidden},
		{"proj-b", "svc-b1", http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/services?projectId="+tc.pid+"&serviceId="+tc.sid, nil)
		req.Header.Set("Authorization", "Bearer "+memberTok1)
		rr := httptest.NewRecorder()
		svcGuard(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("scope(%s, %s) got %d, want %d", tc.pid, tc.sid, rr.Code, tc.want)
		}
	}

	// 4. Active Sessions & Ownership Isolation (IDOR check)
	authHandler := NewHandler(authSvc)
	mux := http.NewServeMux()
	authHandler.RegisterRoutes(mux)
	RegisterAdminRoutes(mux, mw, ac)

	reqList := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	reqList.Header.Set("Authorization", "Bearer "+memberTok1)
	rrList := httptest.NewRecorder()
	mux.ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("list my sessions got %d: %s", rrList.Code, rrList.Body.String())
	}
	var memberSessions []SessionSummary
	if err := json.Unmarshal(rrList.Body.Bytes(), &memberSessions); err != nil {
		t.Fatalf("decode member sessions: %v", err)
	}
	if len(memberSessions) != 2 {
		t.Fatalf("expected 2 member sessions, got %d", len(memberSessions))
	}
	if strings.Contains(rrList.Body.String(), "sha256:") || strings.Contains(rrList.Body.String(), memberTok1) {
		t.Fatal("session list leaked token or token hash")
	}

	// Admin lists sessions to obtain admin session ID, then member must NOT revoke admin session!
	reqAdminSess := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	reqAdminSess.Header.Set("Authorization", "Bearer "+adminTok)
	rrAdminSess := httptest.NewRecorder()
	mux.ServeHTTP(rrAdminSess, reqAdminSess)
	var adminSessions []SessionSummary
	_ = json.Unmarshal(rrAdminSess.Body.Bytes(), &adminSessions)
	if len(adminSessions) != 1 {
		t.Fatalf("expected 1 admin session, got %d", len(adminSessions))
	}

	reqIDOR := httptest.NewRequest(http.MethodDelete, "/api/auth/sessions/"+adminSessions[0].ID, nil)
	reqIDOR.Header.Set("Authorization", "Bearer "+memberTok1)
	rrIDOR := httptest.NewRecorder()
	mux.ServeHTTP(rrIDOR, reqIDOR)
	if rrIDOR.Code != http.StatusNotFound {
		t.Fatalf("member revoking admin session got %d, want 404", rrIDOR.Code)
	}

	// Revoke-others keeps memberTok1 and revokes memberTok2
	reqRevOthers := httptest.NewRequest(http.MethodPost, "/api/auth/sessions/revoke-others", nil)
	reqRevOthers.Header.Set("Authorization", "Bearer "+memberTok1)
	rrRevOthers := httptest.NewRecorder()
	mux.ServeHTTP(rrRevOthers, reqRevOthers)
	if rrRevOthers.Code != http.StatusOK {
		t.Fatalf("revoke-others got %d: %s", rrRevOthers.Code, rrRevOthers.Body.String())
	}
	if s, _, _ := authRepo.GetSession(ctx, memberTok2); s != nil {
		t.Fatal("memberTok2 must be revoked by revoke-others")
	}
	if s, _, _ := authRepo.GetSession(ctx, memberTok1); s == nil {
		t.Fatal("memberTok1 must remain active after revoke-others")
	}

	// 5. Updating member permissions immediately revokes member's active sessions
	updatedPerms := UserPermissions{CanViewMonitoring: true}
	if _, err := ac.UpdateManagedUser(ctx, adminUser.ID, member.ID, UpdateUserRequest{
		Permissions: &updatedPerms,
	}); err != nil {
		t.Fatalf("UpdateManagedUser: %v", err)
	}
	if s, _, _ := authSvc.Login(ctx, member.Email, "friendpass123"); s == nil {
		t.Fatal("expected member account still usable for fresh login")
	}
	if sess, _, _ := authRepo.GetSession(ctx, memberTok1); sess != nil {
		t.Fatal("old member session must be revoked after permission downgrade")
	}

	// 6. Guard against deleting last admin or self
	if err := ac.DeleteManagedUser(ctx, adminUser.ID, adminUser.ID); err == nil {
		t.Fatal("expected error deleting own active admin account")
	}
	if err := ac.DeleteManagedUser(ctx, "other-id", adminUser.ID); err == nil {
		t.Fatal("expected error deleting last remaining admin")
	}
}
