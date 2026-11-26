package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"gopod/pkg/httputil"
)

// AccessChecker provides SQLite-backed multi-user and per-project isolation
// lookups and user management CRUD.
type AccessChecker struct {
	db *sql.DB
}

// NewAccessChecker initializes project membership and user management queries.
func NewAccessChecker(db *sql.DB) *AccessChecker {
	return &AccessChecker{db: db}
}

func (a *AccessChecker) CanAccessProject(ctx context.Context, userID, projectID string) (bool, error) {
	if a == nil || a.db == nil {
		return true, nil
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

func (a *AccessChecker) ListUserProjects(ctx context.Context, userID string) ([]string, error) {
	if a == nil || a.db == nil {
		return []string{}, nil
	}
	rows, err := a.db.QueryContext(ctx, `SELECT project_id FROM project_members WHERE user_id = ? ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err == nil {
			out = append(out, pid)
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
		INSERT INTO project_members (project_id, user_id, role)
		VALUES (?, ?, ?)
		ON CONFLICT(project_id, user_id) DO UPDATE SET role = excluded.role
	`, projectID, userID, role)
	return err
}

func (a *AccessChecker) SetUserProjects(ctx context.Context, userID string, projectIDs []string) error {
	if a == nil || a.db == nil {
		return errors.New("access database unavailable")
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_members WHERE user_id = ?`, userID); err != nil {
		_ = tx.Rollback()
		return err
	}
	seen := make(map[string]bool, len(projectIDs))
	for _, pid := range projectIDs {
		clean := strings.TrimSpace(pid)
		if clean == "" || clean == "system" || seen[clean] {
			continue
		}
		seen[clean] = true
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO project_members (project_id, user_id, role) VALUES (?, ?, 'member')`, clean, userID); err != nil {
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
	rows, err := a.db.QueryContext(ctx, `SELECT id, email, name, role, created_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AdminUserSummary
	for rows.Next() {
		var u AdminUserSummary
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.ProjectIDs = []string{}
		list = append(list, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range list {
		pids, _ := a.ListUserProjects(ctx, list[i].ID)
		if pids != nil {
			list[i].ProjectIDs = pids
		}
	}
	return list, nil
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

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	uid := "usr-" + uuid.New().String()[:8]
	if _, err := a.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, role) VALUES (?, ?, ?, ?, ?)
	`, uid, email, string(hash), name, role); err != nil {
		return nil, err
	}
	if err := a.SetUserProjects(ctx, uid, req.ProjectIDs); err != nil {
		return nil, err
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
	err := a.db.QueryRowContext(ctx, `SELECT name, role FROM users WHERE id = ?`, targetID).Scan(&curName, &curRole)
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
		curRole = nextRole
	}

	if _, err := a.db.ExecContext(ctx, `UPDATE users SET name = ?, role = ? WHERE id = ?`, curName, curRole, targetID); err != nil {
		return nil, err
	}

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
		_, _ = a.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, targetID)
	}

	if req.ProjectIDs != nil {
		if err := a.SetUserProjects(ctx, targetID, req.ProjectIDs); err != nil {
			return nil, err
		}
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
	_, err = a.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, targetID)
	return err
}

// RegisterAdminRoutes exposes user & project membership management endpoints
// guarded strictly by administrator role checks.
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
		if actor != nil {
			actorID = actor.ID
		}
		u, err := ac.UpdateManagedUser(r.Context(), actorID, id, req)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, u)
	}))

	mux.HandleFunc("DELETE /api/users/{id}", mw.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		actor, _ := UserFromContext(r.Context())
		actorID := ""
		if actor != nil {
			actorID = actor.ID
		}
		if err := ac.DeleteManagedUser(r.Context(), actorID, id); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "User removed"})
	}))
}

