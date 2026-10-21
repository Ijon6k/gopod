package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/credentials"
)

type failingSSHKeyProvider struct{}

func (failingSSHKeyProvider) GetPrivateKey(context.Context, string) (string, error) {
	return "", errors.New("key is unreadable")
}

func TestDeployGitFailsWhenSSHKeyUnavailable(t *testing.T) {
	d := NewDeployer(nil, failingSSHKeyProvider{}, nil, t.TempDir())
	_, err := d.deployGit(context.Background(), Service{
		ID:       "svc-ssh",
		GitRepo:  "file:///tmp/does-not-matter.git",
		SSHKeyID: "key-1",
	}, "dep-1", func(string) {})
	if err == nil || !strings.Contains(err.Error(), "load ssh key") {
		t.Fatalf("expected a clear ssh key error, got %v", err)
	}
}

func TestResolveDockerfile(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "Dockerfile"), []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(workspace, "build"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "build", "Containerfile"), []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		configured string
		wantSuffix string
		wantErr    bool
	}{
		{"default", "", "Dockerfile", false},
		{"relative", "./build/Containerfile", filepath.Join("build", "Containerfile"), false},
		{"missing", "missing.Dockerfile", "", true},
		{"directory", "build", "", true},
		{"absolute", "/etc/passwd", "", true},
		{"traversal", "../../etc/passwd", "", true},
	}
	for _, tc := range cases {
		got, err := resolveDockerfile(workspace, tc.configured)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error, got %q", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if !strings.HasSuffix(got, tc.wantSuffix) {
			t.Errorf("%s: got %q, want suffix %q", tc.name, got, tc.wantSuffix)
		}
	}
}

func TestResolveDockerfileRejectsEscapingSymlink(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(workspace, "Dockerfile")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveDockerfile(workspace, "Dockerfile"); err == nil {
		t.Fatal("symlink escaping the repository must be rejected")
	}
}

func TestResolveDockerfileMissingIsTyped(t *testing.T) {
	_, err := resolveDockerfile(t.TempDir(), "Dockerfile")
	if err == nil || !strings.Contains(err.Error(), ErrDockerfileNotFound.Error()) {
		t.Fatalf("expected ErrDockerfileNotFound, got %v", err)
	}
}

func TestNormalizeBuildArgs(t *testing.T) {
	args, rejected := normalizeBuildArgs([]string{
		"NODE_ENV=production",
		"  API_URL=https://example.test  ",
		"NO_EQUALS",
		"=empty",
		"BAD\nVALUE=x",
		"BAD KEY=value",
		"",
	})
	if len(args) != 2 || args[0] != "NODE_ENV=production" || args[1] != "API_URL=https://example.test" {
		t.Fatalf("unexpected accepted args: %#v", args)
	}
	if len(rejected) != 4 {
		t.Fatalf("expected 3 rejected args, got %#v", rejected)
	}
}

func TestWithGitUsername(t *testing.T) {
	got, err := withGitUsername("https://gitlab.com/acme/app.git", "oauth2")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://oauth2@gitlab.com/acme/app.git" {
		t.Fatalf("unexpected clone URL: %q", got)
	}

	got, err = withGitUsername("https://github.com/acme/app.git", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "https://oauth2@") {
		t.Fatalf("expected oauth2 default user, got %q", got)
	}

	if _, err := withGitUsername("http://gitlab.com/acme/app.git", "oauth2"); err == nil {
		t.Fatal("plain HTTP must be rejected")
	}
	if _, err := withGitUsername("git@github.com:acme/app.git", "oauth2"); err == nil {
		t.Fatal("SSH URL must be rejected for token auth")
	}
}

func TestRedactedLoggerRemovesToken(t *testing.T) {
	var lines []string
	logf := redactedLogger(func(line string) { lines = append(lines, line) }, [][2]string{
		{"ghp_secret_value", "***redacted***"},
	})
	logf("fatal: authentication failed for https://x-access-token@example.test (ghp_secret_value)")
	if len(lines) != 1 {
		t.Fatalf("expected one log line, got %d", len(lines))
	}
	if strings.Contains(lines[0], "ghp_secret_value") {
		t.Fatalf("token leaked into the log line: %q", lines[0])
	}
	if !strings.Contains(lines[0], "***redacted***") {
		t.Fatalf("expected the redaction marker, got %q", lines[0])
	}
}

func TestGitCredentialUsername(t *testing.T) {
	cases := []struct {
		cred *credentials.GitToken
		want string
	}{
		{&credentials.GitToken{Provider: "github"}, "x-access-token"},
		{&credentials.GitToken{Provider: "gitlab"}, "oauth2"},
		{&credentials.GitToken{Provider: "gitea"}, "oauth2"},
		{&credentials.GitToken{Provider: "github", Username: "octocat"}, "octocat"},
	}
	for _, tc := range cases {
		if got := gitCredentialUsername(tc.cred); got != tc.want {
			t.Errorf("gitCredentialUsername(%+v) = %q, want %q", tc.cred, got, tc.want)
		}
	}
}
