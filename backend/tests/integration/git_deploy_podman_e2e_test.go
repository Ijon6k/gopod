package integration

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGitDeployPodmanE2E exercises the real Git pipeline against a local
// file:// bare repository and a FROM scratch image, so it needs no registry
// access: clone, Dockerfile build, container start, manual redeploy on a new
// commit, and signed webhook deliveries (duplicate, tampered, deletion, ping,
// and branch-mismatch).
//
//	GOPOD_PODMAN_E2E=1 go test ./tests/integration/ -run TestGitDeployPodmanE2E -v
func TestGitDeployPodmanE2E(t *testing.T) {
	if !podmanE2EEnabled() {
		t.Skip("set GOPOD_PODMAN_E2E=1 with a reachable rootless podman socket to run")
	}

	repoDir := t.TempDir()
	writeE2EFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nimport \"time\"\n\nfunc main() { for { time.Sleep(time.Hour) } }\n")
	writeE2EFile(t, filepath.Join(repoDir, "Dockerfile"), "FROM scratch\nCOPY app /app\nENTRYPOINT [\"/app\"]\n")
	writeE2EFile(t, filepath.Join(repoDir, "version.txt"), "1\n")

	build := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", filepath.Join(repoDir, "app"), "main.go")
	build.Dir = repoDir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build static fixture: %v\n%s", err, out)
	}

	gitE2E(t, repoDir, "init", "-b", "main")
	gitE2E(t, repoDir, "config", "user.email", "e2e@gopod.test")
	gitE2E(t, repoDir, "config", "user.name", "GOPOD E2E")
	gitE2E(t, repoDir, "add", ".")
	gitE2E(t, repoDir, "commit", "-m", "initial commit")

	bareDir := filepath.Join(t.TempDir(), "origin.git")
	gitE2E(t, "", "clone", "--bare", repoDir, bareDir)
	gitE2E(t, repoDir, "remote", "add", "origin", bareDir)
	repoURL := "file://" + bareDir
	sha1 := gitE2E(t, repoDir, "rev-parse", "HEAD")

	dbPath := filepath.Join(t.TempDir(), "gopod.db")
	handler := newIntegrationServer(t, dbPath, "http://127.0.0.1:9")
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	client := loginAdmin(t, ts.URL)

	code, data := client.post(t, "/api/projects", []byte(`{"name":"git-e2e","description":"Git pipeline E2E"}`))
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("create project: %d %s", code, trunc(data))
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &project); err != nil || project.ID == "" {
		t.Fatalf("project payload incomplete: %v / %s", err, trunc(data))
	}

	svcBody, _ := json.Marshal(map[string]any{
		"projectId":       project.ID,
		"name":            "git-e2e",
		"type":            "application",
		"sourceType":      "git",
		"source":          repoURL,
		"branch":          "main",
		"buildType":       "dockerfile",
		"dockerfilePath":  "./Dockerfile",
		"port":            8080,
		"restartPolicy":   "no",
		"autoDeploy":      true,
		"webhookProvider": "github",
		"envVars":         []map[string]any{},
	})
	code, data = client.post(t, "/api/services", svcBody)
	if code != http.StatusCreated {
		t.Fatalf("create service: %d %s", code, trunc(data))
	}
	var svc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &svc); err != nil || svc.ID == "" {
		t.Fatalf("service payload incomplete: %v / %s", err, trunc(data))
	}
	imageTag := fmt.Sprintf("gopod-%s-git-e2e:latest", strings.ToLower(project.ID))
	t.Cleanup(func() {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/services/"+svc.ID+"?delete_volumes=true", nil)
		if err == nil {
			if resp, err := client.http.Do(req); err == nil {
				resp.Body.Close()
			}
		}
		_, _ = podmanCLI(t, 30*time.Second, "rmi", "-f", imageTag)
	})

	code, data = client.get(t, "/api/services/"+svc.ID+"/webhook")
	if code != http.StatusOK {
		t.Fatalf("reveal webhook: %d %s", code, trunc(data))
	}
	var hook struct {
		WebhookToken     string `json:"webhookToken"`
		HasWebhook       bool   `json:"hasWebhook"`
		HasWebhookSecret bool   `json:"hasWebhookSecret"`
	}
	if err := json.Unmarshal(data, &hook); err != nil || !hook.HasWebhook || hook.WebhookToken == "" {
		t.Fatalf("webhook token missing: %v / %s", err, trunc(data))
	}
	if hook.HasWebhookSecret {
		t.Fatal("new service must not report a signing secret before one is generated")
	}

	code, data = client.post(t, "/api/services/"+svc.ID+"/webhook/secret", nil)
	if code != http.StatusOK {
		t.Fatalf("rotate webhook secret: %d %s", code, trunc(data))
	}
	var secretResp struct {
		WebhookSecret string `json:"webhookSecret"`
	}
	if err := json.Unmarshal(data, &secretResp); err != nil || secretResp.WebhookSecret == "" {
		t.Fatalf("webhook secret missing: %v / %s", err, trunc(data))
	}
	webhookSecret := secretResp.WebhookSecret

	// ── Deploy #1: initial commit ────────────────────────────────────────────
	first := deployAndWait(t, client, svc.ID, 0)
	if first.CommitHash != shortSHA(t, repoDir, sha1) {
		t.Fatalf("deploy #1 commit = %q, want %q", first.CommitHash, shortSHA(t, repoDir, sha1))
	}
	if first.Trigger != "manual" {
		t.Fatalf("deploy #1 trigger = %q", first.Trigger)
	}
	assertExactlyOneOwnedRunning(t, svc.ID)

	// ── Deploy #2: manual redeploy picks up a new commit ─────────────────────
	writeE2EFile(t, filepath.Join(repoDir, "version.txt"), "2\n")
	sha2 := commitAndPush(t, repoDir, "second commit")
	second := deployAndWait(t, client, svc.ID, 1)
	if second.CommitHash != shortSHA(t, repoDir, sha2) {
		t.Fatalf("deploy #2 commit = %q, want %q", second.CommitHash, shortSHA(t, repoDir, sha2))
	}
	assertExactlyOneOwnedRunning(t, svc.ID)

	// ── Deploy #3: signed GitHub push, duplicate delivery deduped ────────────
	writeE2EFile(t, filepath.Join(repoDir, "version.txt"), "3\n")
	sha3 := commitAndPush(t, repoDir, "third commit")
	payload, _ := json.Marshal(map[string]any{
		"ref":    "refs/heads/main",
		"before": sha2,
		"after":  sha3,
		"commits": []map[string]any{{
			"id":       sha3,
			"message":  "third commit",
			"modified": []string{"version.txt"},
		}},
	})
	signature := githubSignature(webhookSecret, payload)
	delivery := "delivery-" + sha3[:12]

	before := len(listGitDeployments(t, client, svc.ID))
	code, data = postWebhook(t, client, hook.WebhookToken, "push", delivery, signature, payload)
	if code != http.StatusAccepted {
		t.Fatalf("signed webhook expected 202, got %d: %s", code, trunc(data))
	}
	code, data = postWebhook(t, client, hook.WebhookToken, "push", delivery, signature, payload)
	if code != http.StatusOK || !strings.Contains(string(data), "Duplicate") {
		t.Fatalf("duplicate delivery expected 200 ignored, got %d: %s", code, trunc(data))
	}
	third := waitDeploymentAfter(t, client, svc.ID, before)
	if third.CommitHash != shortSHA(t, repoDir, sha3) {
		t.Fatalf("webhook deploy commit = %q, want %q", third.CommitHash, shortSHA(t, repoDir, sha3))
	}
	if third.Trigger != "webhook" {
		t.Fatalf("webhook deploy trigger = %q", third.Trigger)
	}
	if len(listGitDeployments(t, client, svc.ID)) != before+1 {
		t.Fatal("duplicate delivery must not create a second deployment")
	}
	assertExactlyOneOwnedRunning(t, svc.ID)

	// ── Rejected and ignored deliveries create no deployment ─────────────────
	assertNoNewDeployment := func(label string, body []byte, event, delivery, signature string, want int) {
		t.Helper()
		previous := len(listGitDeployments(t, client, svc.ID))
		code, data := postWebhook(t, client, hook.WebhookToken, event, delivery, signature, body)
		if code != want {
			t.Fatalf("%s: expected %d, got %d: %s", label, want, code, trunc(data))
		}
		time.Sleep(1200 * time.Millisecond)
		if got := len(listGitDeployments(t, client, svc.ID)); got != previous {
			t.Fatalf("%s: deployment count changed from %d to %d", label, previous, got)
		}
	}

	tamperedBody := bytes.Replace(payload, []byte("third commit"), []byte("third commiT"), 1)
	assertNoNewDeployment("tampered signature", tamperedBody, "push", "delivery-tampered", signature, http.StatusUnauthorized)

	deleted, _ := json.Marshal(map[string]any{
		"ref":     "refs/heads/main",
		"before":  sha3,
		"after":   "0000000000000000000000000000000000000000",
		"deleted": true,
	})
	assertNoNewDeployment("branch deletion", deleted, "push", "delivery-deleted",
		githubSignature(webhookSecret, deleted), http.StatusOK)

	mismatch, _ := json.Marshal(map[string]any{
		"ref":    "refs/heads/dev",
		"before": sha2,
		"after":  sha3,
	})
	assertNoNewDeployment("branch mismatch", mismatch, "push", "delivery-dev",
		githubSignature(webhookSecret, mismatch), http.StatusOK)

	ping := []byte(`{"zen":"Keep it logically awesome."}`)
	assertNoNewDeployment("ping event", ping, "ping", "delivery-ping",
		githubSignature(webhookSecret, ping), http.StatusOK)
}

type gitDeploymentEntry struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Trigger    string `json:"trigger"`
	CommitHash string `json:"commitHash"`
}

func writeE2EFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func gitE2E(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitAndPush(t *testing.T, repoDir, message string) string {
	t.Helper()
	gitE2E(t, repoDir, "add", "-A")
	gitE2E(t, repoDir, "commit", "-m", message)
	gitE2E(t, repoDir, "push", "origin", "main")
	return gitE2E(t, repoDir, "rev-parse", "HEAD")
}

func shortSHA(t *testing.T, repoDir, sha string) string {
	t.Helper()
	return gitE2E(t, repoDir, "rev-parse", "--short", sha)
}

func listGitDeployments(t *testing.T, c *e2eClient, svcID string) []gitDeploymentEntry {
	t.Helper()
	code, data := c.get(t, "/api/services/"+svcID+"/deployments")
	if code != http.StatusOK {
		t.Fatalf("list deployments: %d %s", code, trunc(data))
	}
	var deps []gitDeploymentEntry
	if err := json.Unmarshal(data, &deps); err != nil {
		t.Fatalf("deployment JSON: %v / %s", err, trunc(data))
	}
	return deps
}

// deployAndWait triggers a manual deploy and waits for it to finish.
func deployAndWait(t *testing.T, c *e2eClient, svcID string, previous int) gitDeploymentEntry {
	t.Helper()
	code, data := c.post(t, "/api/services/"+svcID+"/deploy", []byte(`{"trigger":"manual"}`))
	if code != http.StatusAccepted {
		t.Fatalf("deploy: expected 202, got %d: %s", code, trunc(data))
	}
	return waitDeploymentAfter(t, c, svcID, previous)
}

// waitDeploymentAfter waits until at least previous+1 deployments exist and
// every visible deployment is terminal, then returns the newest one.
func waitDeploymentAfter(t *testing.T, c *e2eClient, svcID string, previous int) gitDeploymentEntry {
	t.Helper()
	deadline := time.Now().Add(e2eDeployTimeout)
	for time.Now().Before(deadline) {
		deps := listGitDeployments(t, c, svcID)
		if len(deps) > previous {
			allTerminal := true
			for _, d := range deps {
				if !deployTerminal(d.Status) {
					allTerminal = false
					break
				}
			}
			if allTerminal {
				if deps[0].Status != "success" {
					logs := ""
					if code, raw := c.get(t, "/api/deployments/"+deps[0].ID+"/logs"); code == http.StatusOK {
						logs = trunc(raw)
					}
					t.Fatalf("deployment %s finished with %q\n%s", deps[0].ID, deps[0].Status, logs)
				}
				return deps[0]
			}
		}
		time.Sleep(e2ePollInterval)
	}
	t.Fatalf("deployment did not finish within %s", e2eDeployTimeout)
	return gitDeploymentEntry{}
}

func githubSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func postWebhook(t *testing.T, c *e2eClient, token, event, delivery, signature string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, c.base+"/api/deploy/webhook/"+token, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build webhook request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-GitHub-Delivery", delivery)
	if signature != "" {
		req.Header.Set("X-Hub-Signature-256", signature)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("webhook request: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}
