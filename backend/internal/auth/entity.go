package auth

import (
	"strings"
	"time"
)

// User represents an administrator or team member.
type User struct {
	ID           string          `json:"id"`
	Email        string          `json:"email"`
	PasswordHash string          `json:"-"`
	Name         string          `json:"name"`
	Role         string          `json:"role"`
	Permissions  UserPermissions `json:"permissions"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// IsAdmin reports whether the user holds administrator privileges.
func (u *User) IsAdmin() bool {
	return u != nil && strings.EqualFold(strings.TrimSpace(u.Role), "admin")
}

// ProjectScope scopes a non-admin user to a specific project and optionally to a
// subset of services within that project (empty ServiceIDs means all services in
// the project).
type ProjectScope struct {
	ProjectID  string   `json:"projectId"`
	ServiceIDs []string `json:"serviceIds"`
}

// ProjectMember assigns a non-admin user to a specific project workspace.
type ProjectMember struct {
	ProjectID  string    `json:"projectId"`
	UserID     string    `json:"userId"`
	Role       string    `json:"role"`
	ServiceIDs []string  `json:"serviceIds,omitempty"`
	Email      string    `json:"email,omitempty"`
	Name       string    `json:"name,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// AdminUserSummary is the sanitized admin directory projection of a user.
type AdminUserSummary struct {
	ID            string          `json:"id"`
	Email         string          `json:"email"`
	Name          string          `json:"name"`
	Role          string          `json:"role"`
	Permissions   UserPermissions `json:"permissions"`
	ProjectIDs    []string        `json:"projectIds"`
	ProjectScopes []ProjectScope  `json:"projectScopes"`
	CreatedAt     time.Time       `json:"createdAt"`
}

// CreateUserRequest defines an admin-created user account payload.
type CreateUserRequest struct {
	Name          string           `json:"name"`
	Email         string           `json:"email"`
	Password      string           `json:"password"`
	Role          string           `json:"role"`
	Permissions   *UserPermissions `json:"permissions,omitempty"`
	ProjectIDs    []string         `json:"projectIds"`
	ProjectScopes []ProjectScope   `json:"projectScopes,omitempty"`
}

// UpdateUserRequest defines user role, permissions, password reset, and scope changes.
type UpdateUserRequest struct {
	Name          *string          `json:"name"`
	Role          *string          `json:"role"`
	Permissions   *UserPermissions `json:"permissions,omitempty"`
	Password      string           `json:"password"`
	ProjectIDs    []string         `json:"projectIds"`
	ProjectScopes []ProjectScope   `json:"projectScopes,omitempty"`
}

// Session represents an authenticated web session. Token is never serialized.
type Session struct {
	ID         string    `json:"id"`
	Token      string    `json:"-"`
	UserID     string    `json:"userId"`
	IPAddress  string    `json:"ipAddress"`
	UserAgent  string    `json:"userAgent"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

// SessionSummary is the redacted representation returned by session management endpoints.
type SessionSummary struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	UserName   string    `json:"userName,omitempty"`
	UserEmail  string    `json:"userEmail,omitempty"`
	UserRole   string    `json:"userRole,omitempty"`
	IPAddress  string    `json:"ipAddress"`
	UserAgent  string    `json:"userAgent"`
	Device     string    `json:"device"`
	IsCurrent  bool      `json:"isCurrent"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

// LoginRequest defines credentials payload.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SetupRequest defines initial admin account creation payload.
type SetupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// AuthResponse returns sanitized user details.
type AuthResponse struct {
	Token string `json:"token,omitempty"`
	User  User   `json:"user"`
}
