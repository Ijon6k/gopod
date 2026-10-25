package services

import (
	"strings"
	"testing"
)

func TestValidateGitBranch(t *testing.T) {
	valid := []string{
		"main", "master", "develop",
		"feature/secret-rotation", "release-1.2.3",
		"user_name/branch+hotfix", "a.b/c.d",
		// Branches git itself accepts that an over-strict allowlist rejected:
		"hotfix/#42", "release@2", "_wip", "feature/ähm", "a;b",
	}
	for _, branch := range valid {
		if err := ValidateGitBranch(branch); err != nil {
			t.Errorf("branch %q must be accepted, got %v", branch, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"-x",
		"--upload-pack=/bin/sh",
		"feature/../etc",
		"branch with space",
		"branch;rm -rf /",
		"feature/x.lock",
		"x.lock",
		"trail/",
		"trailing.",
		"refs/heads/ok?",
		"back\\slash",
		".hidden",
		"a/.hidden",
		"a..b",
		"a@{b",
		"a//b",
		"trail\tine",
		strings.Repeat("a", 300),
	}
	for _, branch := range invalid {
		if err := ValidateGitBranch(branch); err == nil {
			t.Errorf("branch %q must be rejected", branch)
		}
	}
}

func TestValidDeploymentLogID(t *testing.T) {
	valid := []string{
		"dep-1730000000-abcd1234",
		"start-1730000000",
		"demote-1730000000",
		"group-pod-1730000000",
		"ungroup-pod-1730000000",
		"member-sync-1730000000123456789",
	}
	for _, id := range valid {
		if !ValidDeploymentLogID(id) {
			t.Errorf("id %q must be accepted", id)
		}
	}

	invalid := []string{
		"",
		"../etc/passwd",
		"dep/../other",
		"dep-1.log",
		"-leading",
		"dep 1",
		"dep%2Fetc",
		"sub/dir",
		"dep.1",
		strings.Repeat("a", 200),
	}
	for _, id := range invalid {
		if ValidDeploymentLogID(id) {
			t.Errorf("id %q must be rejected (path traversal shape)", id)
		}
	}
}

// Regression tests for the SEC-7 identifier charset (SEC-7: client-supplied
// Service.ID / ProjectID used directly in filesystem paths incl.
// os.RemoveAll). Every valid generated id shape must pass; every traversal,
// flag-shaped, separator-bearing or spaced value must fail.
func TestValidateIdentifierAcceptsGeneratedShapes(t *testing.T) {
	for _, id := range []string{
		"proj-1_web-app-a1b2c3d4",
		"nginx.web",
		"a",
		"svc.2-b_3",
	} {
		if err := ValidateIdentifier("service id", id); err != nil {
			t.Errorf("id %q must be accepted: %v", id, err)
		}
	}
}

func TestValidateIdentifierRejectsPathFlagAndWhitespace(t *testing.T) {
	for _, id := range []string{
		"",
		"../Victim",
		"..",
		".",
		"a/b",
		"/abs",
		"--all",
		"-rf",
		"my app",
		"a b",
		"a\tb",
		"a\nb",
		"a..b",
		strings.Repeat("a", 200),
	} {
		if err := ValidateIdentifier("service id", id); err == nil {
			t.Errorf("id %q must be rejected", id)
		}
	}
}
