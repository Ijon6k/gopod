package services

import (
	"strings"
	"testing"

	"gopod/internal/podman"
)

// TestSanitizedPodmanRunArgsMasksDeclaredSecrets pins the deployment-log
// contract: env values flagged secret are masked by exact key, whatever the
// key is named, while the name heuristic stays as a fallback for
// compose-derived values.
func TestSanitizedPodmanRunArgsMasksDeclaredSecrets(t *testing.T) {
	opts := podman.RunContainerOptions{
		Name:  "demo",
		Image: "alpine:latest",
		Env: map[string]string{
			"DATABASE_URL":   "postgres://user:pw@db/app",
			"FOO":            "plain-value",
			"MYSQL_PASSWORD": "heuristic-secret",
		},
	}

	rendered := sanitizedPodmanRunArgs(opts, map[string]bool{"DATABASE_URL": true})

	if strings.Contains(rendered, "postgres://user:pw@db/app") {
		t.Fatalf("declared secret leaked into the log line: %s", rendered)
	}
	if !strings.Contains(rendered, "DATABASE_URL=***redacted***") {
		t.Fatalf("declared secret must be masked: %s", rendered)
	}
	if !strings.Contains(rendered, "FOO=plain-value") {
		t.Fatalf("non-secret value must stay readable: %s", rendered)
	}
	if strings.Contains(rendered, "heuristic-secret") {
		t.Fatalf("name heuristic must keep masking password-like keys: %s", rendered)
	}
}

// TestSecretEnvKeySet collects only flagged, non-empty keys.
func TestSecretEnvKeySet(t *testing.T) {
	set := SecretEnvKeySet(Service{
		EnvVars: []EnvVar{
			{Key: "A", Value: "x", Secret: true},
			{Key: "B", Value: "y", Secret: false},
			{Key: "  ", Value: "z", Secret: true},
			{Key: "C", Value: "w", Secret: true},
		},
	})
	if len(set) != 2 || !set["A"] || !set["C"] {
		t.Fatalf("unexpected secret key set: %v", set)
	}
}

// TestRedactCloneURLStripsCredentials: tokens embedded in clone URLs must not
// reach deployment logs; URLs without userinfo stay untouched.
func TestRedactCloneURLStripsCredentials(t *testing.T) {
	cases := map[string]string{
		"https://user:tok3n@github.com/org/repo.git": "https://github.com/org/repo.git",
		"https://github.com/org/repo.git":            "https://github.com/org/repo.git",
		"git@github.com:org/repo.git":                "git@github.com:org/repo.git",
	}
	for in, want := range cases {
		if got := redactCloneURL(in); got != want {
			t.Errorf("redactCloneURL(%q) = %q, want %q", in, got, want)
		}
	}
}
