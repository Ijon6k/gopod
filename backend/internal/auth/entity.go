package auth

import (
	"strings"
	"time"
)

// User represents an administrator or team member.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
}

// IsAdmin reports whether the user holds administrator privileges.
func (u *User) IsAdmin() bool {
	return u != nil && strings.EqualFold(strings.TrimSpace(u.Role), "admin")
}

// ProjectMember assigns a non-admin user to a specific project workspace.
type ProjectMember struct {
	ProjectID string    `json:"projectId"`
	UserID    string    `json:"userId"`
	Role      string    `json:"role"`
	Email     string    `json:"email,omitempty"`
	Name      string    `json:"name,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// AdminUserSummary is the sanitized admin directory projection of a user.
type AdminUserSummary struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	ProjectIDs []string  `json:"projectIds"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CreateUserRequest defines an admin-created user account payload.
type CreateUserRequest struct {
	Name       string   `json:"name"`
	Email      string   `json:"email"`
	Password   string   `json:"password"`
	Role       string   `json:"role"`
	ProjectIDs []string `json:"projectIds"`
}

// UpdateUserRequest defines user role, password reset, and project scope changes.
type UpdateUserRequest struct {
	Name       *string  `json:"name"`
	Role       *string  `json:"role"`
	Password   string   `json:"password"`
	ProjectIDs []string `json:"projectIds"`
}

// Session represents an authenticated web session.
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
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

// AuthResponse returns session token and sanitized user details.
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
