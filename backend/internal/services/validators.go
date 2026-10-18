package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
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
