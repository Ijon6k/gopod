package services

import (
	"context"
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

func TestValidateGitURL(t *testing.T) {
	valid := []string{
		"https://github.com/user/repo.git",
		"http://gitlab.local/user/repo",
		"ssh://git@github.com:22/user/repo.git",
		"git://github.com/user/repo.git",
		"git@github.com:user/repo.git",
		"git@gitlab.com:org/subgroup/repo.git",
	}
	for _, u := range valid {
		if err := ValidateGitURL(u); err != nil {
			t.Errorf("git URL %q must be accepted, got %v", u, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"-oProxyCommand=calc.exe",
		"--upload-pack=evil",
		"file:///etc/passwd",
		"ext::sh -c evil",
		"fd::1",
		"git-lfs::https://evil.com",
		"https://",
		"repo with space",
	}
	for _, u := range invalid {
		if err := ValidateGitURL(u); err == nil {
			t.Errorf("git URL %q must be rejected", u)
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

func TestValidateContainerImageRejectsLeadingDash(t *testing.T) {
	if err := ValidateContainerImage("--privileged"); err == nil {
		t.Fatal("expected leading-dash image reference to be rejected")
	}
}

func TestGenerateQuadletPreservesHostBindMountsAndNoNewPrivileges(t *testing.T) {
	svc := Service{
		ID:        "svc-1",
		ProjectID: "proj-1",
		Name:      "pg",
		Image:     "postgres:16-alpine",
		Advanced:  &ServiceAdvancedConfig{},
	}
	svc.Advanced.Security.NoNewPrivileges = true
	svc.Advanced.Security.AppArmorProfile = "docker-default\nExecStart=/bin/sh"
	svc.Advanced.Storage.Volumes = []VolumeMountConfig{
		{Source: "/mnt/ssd/pgdata", Target: "/var/lib/postgresql/data", Options: "Z"},
		{Source: "pg_named", Target: "/var/lib/other", Options: ""},
	}
	d := &Deployer{}
	unit := d.generateDefaultQuadletContent(context.Background(), svc, "pg")
	if !strings.Contains(unit, "Volume=/mnt/ssd/pgdata:/var/lib/postgresql/data:Z") {
		t.Fatalf("expected absolute host bind mount preserved, got:\n%s", unit)
	}
	if !strings.Contains(unit, "Volume=pg_named:/var/lib/other:Z") {
		t.Fatalf("expected named volume preserved, got:\n%s", unit)
	}
	if !strings.Contains(unit, "NoNewPrivileges=true") {
		t.Fatalf("expected NoNewPrivileges=true emitted, got:\n%s", unit)
	}
	if strings.Contains(unit, "\nExecStart=") {
		t.Fatalf("expected newline injection stripped, got:\n%s", unit)
	}
}

func TestRedactDeploymentSnapshotMasksSecrets(t *testing.T) {
	raw := `{"id":"s1","projectId":"p1","name":"web","webhookSecret":"whsec_123","envVars":[{"key":"API_KEY","value":"topsecret","secret":true}]}`
	dep := redactDeploymentSnapshot(Deployment{ID: "d1", ConfigSnapshot: raw})
	if strings.Contains(dep.ConfigSnapshot, "topsecret") || strings.Contains(dep.ConfigSnapshot, "whsec_123") {
		t.Fatalf("expected deployment snapshot secrets redacted, got: %s", dep.ConfigSnapshot)
	}
}
