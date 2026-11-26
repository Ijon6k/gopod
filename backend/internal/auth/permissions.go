package auth

import (
	"encoding/json"
	"strings"
)

// Permission identifies a fine-grained capability assignable to non-admin users.
type Permission string

const (
	PermissionCreateProjects Permission = "canCreateProjects"
	PermissionDeleteProjects Permission = "canDeleteProjects"
	PermissionCreateServices Permission = "canCreateServices"
	PermissionDeployServices Permission = "canDeployServices"
	PermissionDeleteServices Permission = "canDeleteServices"
	PermissionRevealSecrets  Permission = "canRevealSecrets"
	PermissionManageDomains  Permission = "canManageDomains"
	PermissionManageBackups  Permission = "canManageBackups"
	PermissionAccessRuntime  Permission = "canAccessRuntime"
	PermissionControlRuntime Permission = "canControlRuntime"
	PermissionViewMonitoring Permission = "canViewMonitoring"
	PermissionViewActivity   Permission = "canViewActivity"
)

// UserPermissions defines fine-grained capabilities for non-admin accounts.
// All fields default to false (principle of least privilege). Administrators
// implicitly hold every capability regardless of stored flags.
type UserPermissions struct {
	CanCreateProjects bool `json:"canCreateProjects"`
	CanDeleteProjects bool `json:"canDeleteProjects"`
	CanCreateServices bool `json:"canCreateServices"`
	CanDeployServices bool `json:"canDeployServices"`
	CanDeleteServices bool `json:"canDeleteServices"`
	CanRevealSecrets  bool `json:"canRevealSecrets"`
	CanManageDomains  bool `json:"canManageDomains"`
	CanManageBackups  bool `json:"canManageBackups"`
	CanAccessRuntime  bool `json:"canAccessRuntime"`
	CanControlRuntime bool `json:"canControlRuntime"`
	CanViewMonitoring bool `json:"canViewMonitoring"`
	CanViewActivity   bool `json:"canViewActivity"`
}

// DefaultMemberPermissions returns a least-privilege permission set (all false).
func DefaultMemberPermissions() UserPermissions {
	return UserPermissions{}
}

// AllPermissions returns a full capability set (effective permissions for admin).
func AllPermissions() UserPermissions {
	return UserPermissions{
		CanCreateProjects: true,
		CanDeleteProjects: true,
		CanCreateServices: true,
		CanDeployServices: true,
		CanDeleteServices: true,
		CanRevealSecrets:  true,
		CanManageDomains:  true,
		CanManageBackups:  true,
		CanAccessRuntime:  true,
		CanControlRuntime: true,
		CanViewMonitoring: true,
		CanViewActivity:   true,
	}
}

// ParsePermissions decodes stored JSON permissions safely (fail-closed on bad JSON).
func ParsePermissions(raw string) UserPermissions {
	var p UserPermissions
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return p
	}
	_ = json.Unmarshal([]byte(trimmed), &p)
	// Controlling runtime always implies read visibility of runtime resources.
	if p.CanControlRuntime {
		p.CanAccessRuntime = true
	}
	return p
}

// Normalize enforces permission invariants (fail-closed + prerequisite implication).
func (p UserPermissions) Normalize() UserPermissions {
	if p.CanControlRuntime {
		p.CanAccessRuntime = true
	}
	return p
}

// MarshalJSONString encodes normalized permissions as compact JSON.
func (p UserPermissions) MarshalJSONString() string {
	b, err := json.Marshal(p.Normalize())
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Has reports whether the capability set grants permission p.
func (p UserPermissions) Has(perm Permission) bool {
	switch perm {
	case PermissionCreateProjects:
		return p.CanCreateProjects
	case PermissionDeleteProjects:
		return p.CanDeleteProjects
	case PermissionCreateServices:
		return p.CanCreateServices
	case PermissionDeployServices:
		return p.CanDeployServices
	case PermissionDeleteServices:
		return p.CanDeleteServices
	case PermissionRevealSecrets:
		return p.CanRevealSecrets
	case PermissionManageDomains:
		return p.CanManageDomains
	case PermissionManageBackups:
		return p.CanManageBackups
	case PermissionAccessRuntime:
		return p.CanAccessRuntime || p.CanControlRuntime
	case PermissionControlRuntime:
		return p.CanControlRuntime
	case PermissionViewMonitoring:
		return p.CanViewMonitoring
	case PermissionViewActivity:
		return p.CanViewActivity
	default:
		return false
	}
}

// EffectivePermissions resolves the active capability set for a user account.
func (u *User) EffectivePermissions() UserPermissions {
	if u == nil {
		return DefaultMemberPermissions()
	}
	if u.IsAdmin() {
		return AllPermissions()
	}
	return u.Permissions.Normalize()
}

// HasPermission reports whether user holds the requested fine-grained permission.
func (u *User) HasPermission(perm Permission) bool {
	if u == nil {
		return false
	}
	if u.IsAdmin() {
		return true
	}
	return u.Permissions.Has(perm)
}

