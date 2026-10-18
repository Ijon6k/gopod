package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQuadletAPIRedactsServiceSecrets proves the wiring end to end: env values
// the service API masks must not leak through the Quadlet endpoints, while the
// unit file on disk keeps the real value because systemd needs it.
func TestQuadletAPIRedactsServiceSecrets(t *testing.T) {
	quadDir := filepath.Join(t.TempDir(), "quadlets")
	if err := os.MkdirAll(quadDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("QUADLET_DIR", quadDir)

	handler, token := setupAuthedServer(t)

	projBody, _ := json.Marshal(map[string]string{"name": "Quadlet Redaction"})
	rec := doAuthedJSON(t, handler, "POST", "/api/projects", projBody, token)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create project: %d %s", rec.Code, rec.Body.String())
	}
	var proj struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&proj); err != nil || proj.ID == "" {
		t.Fatalf("decode project: %v", err)
	}

	svcBody, _ := json.Marshal(map[string]interface{}{
		"projectId": proj.ID,
		"name":      "quadlet-secret-app",
		"type":      "image",
		"image":     "nginx:alpine",
		"envVars": []map[string]interface{}{
			{"key": "API_SECRET", "value": "unit-leak-canary", "secret": true},
			{"key": "PUBLIC_FLAG", "value": "visible", "secret": false},
		},
	})
	rec = doAuthedJSON(t, handler, "POST", "/api/services", svcBody, token)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create service: %d %s", rec.Code, rec.Body.String())
	}

	// Unit file as the deployer would write it.
	unitContent := "[Unit]\nDescription=demo\n[Container]\nImage=nginx:alpine\n" +
		`Environment="API_SECRET=unit-leak-canary" "PUBLIC_FLAG=visible"` + "\n"
	unitPath := filepath.Join(quadDir, "quadlet-secret-app.container")
	if err := os.WriteFile(unitPath, []byte(unitContent), 0o600); err != nil {
		t.Fatalf("write unit: %v", err)
	}

	// List endpoint: secret masked, plain neighbor intact.
	rec = doAuthedJSON(t, handler, "GET", "/api/quadlets", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list quadlets: %d %s", rec.Code, rec.Body.String())
	}
	listRaw := rec.Body.String()
	if strings.Contains(listRaw, "unit-leak-canary") {
		t.Fatalf("list endpoint leaked the secret value: %s", listRaw)
	}
	if !strings.Contains(listRaw, "API_SECRET=••••••") {
		t.Fatalf("list endpoint must mask the secret value: %s", listRaw)
	}
	if !strings.Contains(listRaw, "PUBLIC_FLAG=visible") {
		t.Fatalf("non-secret value must stay visible in the list: %s", listRaw)
	}

	// Content endpoint: same contract.
	rec = doAuthedJSON(t, handler, "GET", "/api/quadlets/quadlet-secret-app.container/content", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("quadlet content: %d %s", rec.Code, rec.Body.String())
	}
	contentRaw := rec.Body.String()
	if strings.Contains(contentRaw, "unit-leak-canary") {
		t.Fatalf("content endpoint leaked the secret value: %s", contentRaw)
	}
	if !strings.Contains(contentRaw, "API_SECRET=••••••") {
		t.Fatalf("content endpoint must mask the secret value: %s", contentRaw)
	}
	if !strings.Contains(contentRaw, "PUBLIC_FLAG=visible") {
		t.Fatalf("non-secret value must stay visible in the content: %s", contentRaw)
	}

	// The file on disk keeps the real value.
	raw, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("read unit: %v", err)
	}
	if !strings.Contains(string(raw), "unit-leak-canary") {
		t.Fatalf("on-disk unit must keep the real value for systemd")
	}
}
