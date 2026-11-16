package services

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"gopod/internal/podman"
)

// ValidateGitBranch mirrors `git check-ref-format --branch`: it accepts every
// branch git itself accepts (unicode, '#', '@', '+', nested components) while
// rejecting shapes git refuses and the one argv hazard that matters — a
// leading '-' that `git clone -b` would parse as an option. The branch is
// passed as an exec argv element, never through a shell.
func ValidateGitBranch(branch string) error {
	b := strings.TrimSpace(branch)
	if b == "" {
		return errors.New("git branch is required")
	}
	if strings.HasPrefix(b, "-") {
		return fmt.Errorf("git branch %q must not start with '-'", branch)
	}
	if strings.Contains(b, "..") || strings.Contains(b, "@{") || strings.Contains(b, "//") {
		return fmt.Errorf("git branch %q is not a valid ref", branch)
	}
	if strings.HasSuffix(b, "/") || strings.HasSuffix(b, ".") {
		return fmt.Errorf("git branch %q is not a valid ref", branch)
	}
	for _, r := range b {
		if r < 0x20 || r == 0x7f || r == ' ' || strings.ContainsRune("~^:?*[\\", r) {
			return fmt.Errorf("git branch %q contains unsupported characters", branch)
		}
	}
	for _, component := range strings.Split(b, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return fmt.Errorf("git branch %q is not a valid ref", branch)
		}
		// Filesystem limit for a single path component under .git/refs.
		if len(component) > 255 {
			return fmt.Errorf("git branch %q is too long", branch)
		}
	}
	return nil
}

// deploymentLogIDRe matches every deployment log identifier the engine
// generates (dep-..., start-..., demote-..., group-pod-..., member-sync-...,
// ungroup-pod-...). Dots, slashes, percent-escapes and leading dashes are
// excluded, so a crafted id can never escape the deployment log directory.
var deploymentLogIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// ValidDeploymentLogID reports whether a deployment id may be used to address
// a log file.
func ValidDeploymentLogID(id string) bool {
	return deploymentLogIDRe.MatchString(id)
}

// serviceIdentifierRe bounds the identifiers clients may choose for entities
// whose ids are composited into filesystem paths and unit names (git build
// tmpdirs, compose/kube data dirs, static drop directories). One charset
// check at the create boundary — instead of guards sprinkled across every
// sink — plus a filepath.Base equality to reject traversal segments.
var serviceIdentifierRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// ValidateIdentifier rejects arbitrary client-supplied server identifiers.
// Filesystem separators, traversal segments, control characters and any
// leading "-" (so an identifier can never parse as a CLI flag) are all
// impossible under this shape.
func ValidateIdentifier(kind, raw string) error {
	// ".." is refused even mid-string: it is never meaningful in an
	// identifier and any future sink that concatenates instead of joining
	// stays safe.
	if raw == "" || strings.Contains(raw, "..") || !serviceIdentifierRe.MatchString(raw) || filepath.Base(raw) != raw {
		return fmt.Errorf("invalid %s %q: must be a short identifier of letters, digits, '.', '_' or '-'", kind, raw)
	}
	return nil
}

// ValidateVolumeMount checks a structured client-supplied
// Advanced.Storage.Volumes bind-mount before it reaches the deployer. The
// actual policy lives in internal/podman (single source of truth at the
// engine choke point); this wrapper adds the "both halves required" rule for
// the structured form.
func ValidateVolumeMount(source, target string) error {
	if strings.ContainsAny(source, "\x00\n\r:") {
		return errors.New("volume source contains invalid characters or colons")
	}
	src := strings.TrimSpace(source)
	dst := strings.TrimSpace(target)
	if src == "" || dst == "" {
		return errors.New("volume mount requires both source and target")
	}
	if strings.ContainsAny(dst, "\x00\n\r:") {
		return errors.New("volume target contains invalid characters or colons")
	}
	if !strings.HasPrefix(dst, "/") {
		return errors.New("volume target must be an absolute path")
	}
	if err := podman.ValidateVolumeSource(src); err != nil {
		return err
	}
	return nil
}

// ValidateGitURL checks that repoURL uses an allowed scheme (https, http, ssh, git)
// or standard SCP-like SSH syntax (git@host:user/repo.git), and prevents CLI option injection.
func ValidateGitURL(repoURL string) error {
	raw := strings.TrimSpace(repoURL)
	if raw == "" {
		return errors.New("git repository URL is required")
	}
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("git repository URL %q must not start with '-'", raw)
	}
	if strings.Contains(raw, "::") {
		return fmt.Errorf("git repository URL %q contains disallowed remote helper syntax", raw)
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f || r == ' ' {
			return fmt.Errorf("git repository URL %q contains whitespace or control characters", raw)
		}
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid git repository URL: %w", err)
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "https" && scheme != "http" && scheme != "ssh" && scheme != "git" {
			return fmt.Errorf("git repository URL scheme %q is not allowed (must be https, http, ssh, or git)", scheme)
		}
		if u.Host == "" {
			return errors.New("git repository URL missing host")
		}
		return nil
	}
	// SCP-style syntax: [user@]host:path
	colonIdx := strings.Index(raw, ":")
	if colonIdx <= 0 || colonIdx == len(raw)-1 {
		return fmt.Errorf("invalid git repository URL %q", raw)
	}
	hostPart := raw[:colonIdx]
	if strings.Contains(hostPart, "/") || strings.Contains(hostPart, "\\") {
		return fmt.Errorf("invalid git repository URL host %q", hostPart)
	}
	return nil
}
