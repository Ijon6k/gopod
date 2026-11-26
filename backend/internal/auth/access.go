package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"gopod/pkg/httputil"
)

// AccessChecker provides SQLite-backed multi-user, fine-grained RBAC, and
// per-project / per-service isolation lookups plus user management CRUD.
type AccessChecker struct {
	db *sql.DB
}

// NewAccessChecker initializes project membership and user management queries.
func NewAccessChecker(db *sql.DB) *AccessChecker {
	return &AccessChecker{db: db}
}

func (a *AccessChecker) CanAccessProject(ctx context.Context, userID, projectID string) (bool, error) {
	if a == nil || a.db == nil {
		return false, errors.New("access database unavailable")
	}
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(projectID) == "" {
		return false, nil
	}
	var exists int
	err := a.db.QueryRowContext(ctx, `SELECT 1 FROM project_members WHERE project_id = ? AND user_id = ? LIMIT 1`, projectID, userID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

// CanAccessService verifies that userID is assigned to projectID and, when a
// non-empty service whitelist is configured on that membership, that serviceID
// is explicitly included.
func (a *AccessChecker) CanAccessService(ctx context.Context, userID, projectID, serviceID string) (bool, error) {
	if a == nil || a.db == nil {
		return false, errors.New("access database unavailable")
	}
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(projectID) == "" {
		return false, nil
	}
	var rawServices sql.NullString
	err := a.db.QueryRowContext(ctx,
		`SELECT COALESCE(service_ids_json, '[]') FROM project_members WHERE project_id = ? AND user_id = ? LIMIT 1`,
		projectID, userID,
	).Scan(&rawServices)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	allowed := parseServiceIDs(rawServices.String)
	if len(allowed) == 0 {
		// Empty service scope grants access to all services within the assigned project.
		return true, nil
	}
	if strings.TrimSpace(serviceID) == "" {
		return false, nil
	}
	for _, id := range allowed {
		if id == serviceID {
			return true, nil
		}
	}
	return false, nil
}

// AllowedServiceSet reports whether userID has a service-level whitelist inside
// projectID. Restricted=true means only returned IDs are accessible; if user is
// not a member of projectID at all, restricted=true with an empty map is returned.
func (a *AccessChecker) AllowedServiceSet(ctx context.Context, userID, projectID string) (bool, map[string]bool, error) {
	if a == nil || a.db == nil {
		return true, map[string]bool{}, errors.New("access database unavailable")
	}
	var rawServices sql.NullString
	err := a.db.QueryRowContext(ctx,
		`SELECT COALESCE(service_ids_json, '[]') FROM project_members WHERE project_id = ? AND user_id = ? LIMIT 1`,
		projectID, userID,
	).Scan(&rawServices)
	if err == sql.ErrNoRows {
		return true, map[string]bool{}, nil
	}
	if err != nil {
		return true, map[string]bool{}, err
	}
	ids := parseServiceIDs(rawServices.String)
	if len(ids) == 0 {
		return false, nil, nil
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return true, set, nil
}

func parseServiceIDs(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" || trimmed == "null" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(trimmed), &ids); err != nil {
		return nil
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		clean := strings.TrimSpace(id)
		if clean != "" && !seen[clean] {
			seen[clean] = true
			out = append(out, clean)
		}
	}
	return out
}

func (a *AccessChecker) ListUserProjects(ctx context.Context, userID string) ([]string, error) {
	scopes, err := a.ListUserScopes(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, s.ProjectID)
	}
	return out, nil
}

func (a *AccessChecker) ListUserScopes(ctx context.Context, userID string) ([]ProjectScope, error) {
	if a == nil || a.db == nil {
		return []ProjectScope{}, nil
	}
	rows, err := a.db.QueryContext(ctx,
		`SELECT project_id, COALESCE(service_ids_json, '[]') FROM project_members WHERE user_id = ? ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProjectScope, 0)
	for rows.Next() {
		var pid string
		var rawSvcs sql.NullString
		if err := rows.Scan(&pid, &rawSvcs); err == nil {
			svcs := parseServiceIDs(rawSvcs.String)
			if svcs == nil {
				svcs = []string{}
			}
			out = append(out, ProjectScope{
				ProjectID:  pid,
				ServiceIDs: svcs,
			})
		}
	}
	return out, rows.Err()
}

func (a *AccessChecker) GrantProjectAccess(ctx context.Context, projectID, userID, role string) error {
	if a == nil || a.db == nil {
		return nil
	}
	if role == "" {
		role = "member"
	}
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role, service_ids_json)
		VALUES (?, ?, ?, '[]')
		ON CONFLICT(project_id, user_id) DO UPDATE SET role = excluded.role
	`, projectID, userID, role)
	return err
}

func (a *AccessChecker) SetUserProjects(ctx context.Context, userID string, projectIDs []string) error {
	scopes := make([]ProjectScope, 0, len(projectIDs))
	for _, pid := range projectIDs {
		scopes = append(scopes, ProjectScope{ProjectID: pid})
	}
	return a.SetUserScopes(ctx, userID, scopes)
}

func (a *AccessChecker) SetUserScopes(ctx context.Context, userID string, scopes []ProjectScope) error {
	if a == nil || a.db == nil {
		return errors.New("access database unavailable")
	}

	// Validate normalized scopes outside write lock first.
	type normalizedScope struct {
		projectID string
		svcsJSON  string
	}
	var cleaned []normalizedScope
	seenProj := make(map[string]bool, len(scopes))

	for _, sc := range scopes {
		pid := strings.TrimSpace(sc.ProjectID)
		if pid == "" || pid == "system" || seenProj[pid] {
			continue
		}
		var exists int
		if err := a.db.QueryRowContext(ctx, `SELECT 1 FROM projects WHERE id = ? LIMIT 1`, pid).Scan(&exists); err == sql.ErrNoRows {
			return fmt.Errorf("project '%s' does not exist", pid)
		} else if err != nil {
			return err
		}

		var validSvcs []string
		seenSvc := make(map[string]bool, len(sc.ServiceIDs))
		for _, sid := range sc.ServiceIDs {
			cleanSvc := strings.TrimSpace(sid)
			if cleanSvc == "" || seenSvc[cleanSvc] {
				continue
			}
			var svcExists int
			err := a.db.QueryRowContext(ctx, `SELECT 1 FROM services WHERE id = ? AND project_id = ? LIMIT 1`, cleanSvc, pid).Scan(&svcExists)
			if err == sql.ErrNoRows {
				return fmt.Errorf("service '%s' does not exist in project '%s'", cleanSvc, pid)
			} else if err != nil {
				return err
			}
			seenSvc[cleanSvc] = true
			validSvcs = append(validSvcs, cleanSvc)
		}
		svcsBytes, _ := json.Marshal(validSvcs)
		if len(validSvcs) == 0 {
			svcsBytes = []byte("[]")
		}
		seenProj[pid] = true
		cleaned = append(cleaned, normalizedScope{projectID: pid, svcsJSON: string(svcsBytes)})
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_members WHERE user_id = ?`, userID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, sc := range cleaned {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO project_members (project_id, user_id, role, service_ids_json) VALUES (?, ?, 'member', ?)`,
			sc.projectID, userID, sc.svcsJSON,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (a *AccessChecker) ListAdminUsers(ctx context.Context) ([]AdminUserSummary, error) {
	if a == nil || a.db == nil {
		return []AdminUserSummary{}, nil
	}
	rows, err := a.db.QueryContext(ctx, `SELECT id, email, name, role, COALESCE(permissions_json, '{}'), created_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AdminUserSummary
	for rows.Next() {
		var u AdminUserSummary
		var rawPerms sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &rawPerms, &u.CreatedAt); err != nil {
			return nil, err
		}
		if strings.EqualFold(u.Role, "admin") {
			u.Permissions = AllPermissions()
		} else {
			u.Permissions = ParsePermissions(rawPerms.String)
		}
		u.ProjectIDs = []string{}
		u.ProjectScopes = []ProjectScope{}
		list = append(list, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range list {
		scopes, _ := a.ListUserScopes(ctx, list[i].ID)
		if scopes != nil {
			list[i].ProjectScopes = scopes
			pids := make([]string, 0, len(scopes))
			for _, sc := range scopes {
				pids = append(pids, sc.ProjectID)
			}
			list[i].ProjectIDs = pids
		}
	}
	return list, nil
}

func resolveRequestedScopes(projectIDs []string, projectScopes []ProjectScope) []ProjectScope {
	if len(projectScopes) > 0 {
		return projectScopes
	}
	scopes := make([]ProjectScope, 0, len(projectIDs))
	for _, pid := range projectIDs {
		scopes = append(scopes, ProjectScope{ProjectID: pid})
	}
	return scopes
}

func (a *AccessChecker) CreateManagedUser(ctx context.Context, req CreateUserRequest) (*AdminUserSummary, error) {
	if a == nil || a.db == nil {
		return nil, errors.New("user store unavailable")
	}
	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if len(name) < 2 || len(name) > 64 {
		return nil, errors.New("name must be between 2 and 64 characters")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return nil, errors.New("please provide a valid email address")
	}
	if len(req.Password) < 8 || len(req.Password) > 128 {
		return nil, errors.New("password must be between 8 and 128 characters")
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = "member"
	}
	if role != "admin" && role != "member" {
		return nil, errors.New("role must be 'admin' or 'member'")
	}

	var existing int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&existing); err == nil && existing > 0 {
		return nil, errors.New("a user with this email already exists")
	}

	perms := DefaultMemberPermissions()
	if role == "admin" {
		perms = AllPermissions()
	} else if req.Permissions != nil {
		perms = req.Permissions.Normalize()
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	uid := "usr-" + uuid.New().String()[:8]
	if _, err := a.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, role, permissions_json) VALUES (?, ?, ?, ?, ?, ?)
	`, uid, email, string(hash), name, role, perms.MarshalJSONString()); err != nil {
		return nil, err
	}

	if role == "member" {
		scopes := resolveRequestedScopes(req.ProjectIDs, req.ProjectScopes)
		if err := a.SetUserScopes(ctx, uid, scopes); err != nil {
			_, _ = a.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, uid)
			return nil, err
		}
	}

	users, err := a.ListAdminUsers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].ID == uid {
			return &users[i], nil
		}
	}
	return nil, errors.New("created user not found")
}

func (a *AccessChecker) UpdateManagedUser(ctx context.Context, actorID, targetID string, req UpdateUserRequest) (*AdminUserSummary, error) {
	if a == nil || a.db == nil {
		return nil, errors.New("user store unavailable")
	}
	var curName, curRole string
	var curPermsRaw sql.NullString
	err := a.db.QueryRowContext(ctx, `SELECT name, role, COALESCE(permissions_json, '{}') FROM users WHERE id = ?`, targetID).Scan(&curName, &curRole, &curPermsRaw)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if len(n) < 2 || len(n) > 64 {
			return nil, errors.New("name must be between 2 and 64 characters")
		}
		curName = n
	}

	roleChanged := false
	if req.Role != nil {
		nextRole := strings.ToLower(strings.TrimSpace(*req.Role))
		if nextRole != "admin" && nextRole != "member" {
			return nil, errors.New("role must be 'admin' or 'member'")
		}
		if curRole == "admin" && nextRole != "admin" {
			if targetID == actorID {
				return nil, errors.New("cannot demote your own currently active administrator account")
			}
			var adminCount int
			_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&adminCount)
			if adminCount <= 1 {
				return nil, errors.New("cannot demote the last remaining administrator")
			}
		}
		if nextRole != curRole {
			roleChanged = true
		}
		curRole = nextRole
	}

	curPerms := ParsePermissions(curPermsRaw.String)
	permsChanged := false
	if curRole == "admin" {
		nextPerms := AllPermissions()
		if nextPerms != curPerms {
			curPerms = nextPerms
			permsChanged = true
		}
	} else if req.Permissions != nil {
		nextPerms := req.Permissions.Normalize()
		if nextPerms != curPerms {
			curPerms = nextPerms
			permsChanged = true
		}
	}

	if _, err := a.db.ExecContext(ctx,
		`UPDATE users SET name = ?, role = ?, permissions_json = ? WHERE id = ?`,
		curName, curRole, curPerms.MarshalJSONString(), targetID,
	); err != nil {
		return nil, err
	}

	passwordChanged := false
	if req.Password != "" {
		if len(req.Password) < 8 || len(req.Password) > 128 {
			return nil, errors.New("password must be between 8 and 128 characters")
		}
		hash, herr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if herr != nil {
			return nil, herr
		}
		if _, err := a.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), targetID); err != nil {
			return nil, err
		}
		passwordChanged = true
	}

	if curRole == "admin" {
		_ = a.SetUserScopes(ctx, targetID, nil)
	} else if req.ProjectScopes != nil || req.ProjectIDs != nil {
		scopes := resolveRequestedScopes(req.ProjectIDs, req.ProjectScopes)
		if err := a.SetUserScopes(ctx, targetID, scopes); err != nil {
			return nil, err
		}
	}

	// Revoke active sessions on security-sensitive account changes (OWASP A07)
	// unless admin is editing their own account's non-credential details.
	if passwordChanged || ((roleChanged || permsChanged) && targetID != actorID) {
		_, _ = a.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, targetID)
	}

	users, err := a.ListAdminUsers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].ID == targetID {
			return &users[i], nil
		}
	}
	return nil, errors.New("updated user not found")
}

func (a *AccessChecker) DeleteManagedUser(ctx context.Context, actorID, targetID string) error {
	if a == nil || a.db == nil {
		return errors.New("user store unavailable")
	}
	if actorID != "" && actorID == targetID {
		return errors.New("cannot delete your own active account")
	}
	var role string
	err := a.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, targetID).Scan(&role)
	if err == sql.ErrNoRows {
		return errors.New("user not found")
	}
	if err != nil {
		return err
	}
	if role == "admin" {
		var adminCount int
		_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&adminCount)
		if adminCount <= 1 {
			return errors.New("cannot delete the last remaining administrator")
		}
	}
	_, _ = a.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, targetID)
	_, err = a.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, targetID)
	return err
}

// RegisterAdminRoutes exposes user, RBAC, and cluster-wide session management
// endpoints guarded strictly by administrator role checks.
func RegisterAdminRoutes(mux *http.ServeMux, mw *Middleware, ac *AccessChecker) {
	mux.HandleFunc("GET /api/users", mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		users, err := ac.ListAdminUsers(r.Context())
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, users)
	}))

	mux.HandleFunc("POST /api/users", mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil {
			httputil.WriteDecodeError(w, err)
			return
		}
		u, err := ac.CreateManagedUser(r.Context(), req)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if mw != nil && mw.authService != nil && mw.authService.audit != nil {
			actorEmail := "admin"
			if actor, ok := UserFromContext(r.Context()); ok && actor != nil {
				actorEmail = actor.Email
			}
			_ = mw.authService.audit.Record(r.Context(), "user.create", actorEmail, u.Email, "security", ClientIP(r), "success")
		}
		httputil.WriteJSON(w, http.StatusCreated, u)
	}))

	mux.HandleFunc("PUT /api/users/{id}", mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req UpdateUserRequest
		if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil {
			httputil.WriteDecodeError(w, err)
			return
		}
		actor, _ := UserFromContext(r.Context())
		actorID := ""
		actorEmail := "admin"
		if actor != nil {
			actorID = actor.ID
			actorEmail = actor.Email
		}
		u, err := ac.UpdateManagedUser(r.Context(), actorID, id, req)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if mw != nil && mw.authService != nil && mw.authService.audit != nil {
			_ = mw.authService.audit.Record(r.Context(), "user.update", actorEmail, u.Email, "security", ClientIP(r), "success")
		}
		httputil.WriteJSON(w, http.StatusOK, u)
	}))

	mux.HandleFunc("DELETE /api/users/{id}", mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		actor, _ := UserFromContext(r.Context())
		actorID := ""
		actorEmail := "admin"
		if actor != nil {
			actorID = actor.ID
			actorEmail = actor.Email
		}
		if err := ac.DeleteManagedUser(r.Context(), actorID, id); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if mw != nil && mw.authService != nil && mw.authService.audit != nil {
			_ = mw.authService.audit.Record(r.Context(), "user.delete", actorEmail, id, "security", ClientIP(r), "success")
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "User removed"})
	}))

	mux.HandleFunc("GET /api/users/sessions", mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		if mw == nil || mw.authService == nil || mw.authService.repo == nil {
			httputil.WriteError(w, http.StatusServiceUnavailable, "Session store unavailable")
			return
		}
		sessions, err := mw.authService.repo.ListAllSessions(r.Context())
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "Failed to load sessions")
			return
		}
		tok := ExtractTokenHeader(r)
		currentID, _ := mw.authService.repo.GetSessionIDByToken(r.Context(), tok)
		for i := range sessions {
			if currentID != "" && sessions[i].ID == currentID {
				sessions[i].IsCurrent = true
			}
		}
		httputil.WriteJSON(w, http.StatusOK, sessions)
	}))

	mux.HandleFunc("DELETE /api/users/sessions/{id}", mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		if mw == nil || mw.authService == nil || mw.authService.repo == nil {
			httputil.WriteError(w, http.StatusServiceUnavailable, "Session store unavailable")
			return
		}
		sid := strings.TrimSpace(r.PathValue("id"))
		if sid == "" {
			httputil.WriteError(w, http.StatusBadRequest, "Session ID is required")
			return
		}
		tok := ExtractTokenHeader(r)
		currentID, _ := mw.authService.repo.GetSessionIDByToken(r.Context(), tok)
		wasCurrent := currentID != "" && currentID == sid
		deleted, err := mw.authService.repo.DeleteSessionByID(r.Context(), sid, "")
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "Failed to revoke session")
			return
		}
		if !deleted {
			httputil.WriteError(w, http.StatusNotFound, "Session not found")
			return
		}
		if wasCurrent {
			setSessionCookie(w, r, "", -1)
		}
		if mw.authService.audit != nil {
			actorEmail := "admin"
			if actor, ok := UserFromContext(r.Context()); ok && actor != nil {
				actorEmail = actor.Email
			}
			_ = mw.authService.audit.Record(r.Context(), "auth.admin_session_revoked", actorEmail, sid, "security", ClientIP(r), "success")
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "revokedCurrent": wasCurrent})
	}))
}
