package services

import (
	"strings"
	"testing"
)

// These tests validate the generated Quadlet units against the real podman
// quadlet generator (when present). A single unsupported key aborts conversion
// of the whole unit, so the dryrun check is the authoritative guard against
// writing units systemd silently ignores.

func TestBuildPodQuadletBundlePassesQuadletDryRun(t *testing.T) {
	// Negative control: the harness must reject the pre-fix key set, otherwise
	// this test could pass vacuously.
	broken := map[string]string{
		"broken.pod": "# File: broken.pod\n[Unit]\nDescription=broken\n[Pod]\nPodName=pod_broken\nNetwork=gopod-net\nShare=net,uts,ipc\n[Install]\nWantedBy=default.target\n",
	}
	if valid, out, _ := ValidateQuadletFiles(broken); valid {
		t.Fatalf("control unit with unsupported Share= key must fail dryrun, but passed: %s", out)
	}

	s := Service{ID: "proj-web-abc123", Name: "web", ProjectID: "proj"}
	bundle := buildPodQuadletBundle(s, "pod_proj-web-abc123", []string{
		"18401:80",
		"127.0.0.1:18402:5230/udp",
	})

	files := SplitQuadletUnits(bundle, "pod_proj-web-abc123")
	if len(files) == 0 {
		t.Fatalf("bundle produced no unit files:\n%s", bundle)
	}
	valid, out, err := ValidateQuadletFiles(files)
	if err != nil || !valid {
		t.Fatalf("generated .pod bundle must pass quadlet dryrun: valid=%v err=%v out=%s", valid, err, out)
	}
	if strings.Contains(out, "unsupported key") {
		t.Fatalf("generated .pod bundle uses an unsupported [Pod] key: %s", out)
	}
}

func TestGenerateDefaultQuadletContentPassesQuadletDryRun(t *testing.T) {
	d := NewDeployer(nil, nil, nil, t.TempDir())
	s := Service{
		ID:          "svc-multi-1",
		ProjectID:   "proj-multi",
		Name:        "stack",
		ComposeYaml: "services:\n  web:\n    image: nginx:alpine\n  db:\n    image: postgres:16\n",
	}
	content := d.generateDefaultQuadletContent(s, "stack")
	files := SplitQuadletUnits(content, "stack")
	if len(files) == 0 {
		t.Fatalf("generated content produced no unit files:\n%s", content)
	}
	valid, out, err := ValidateQuadletFiles(files)
	if err != nil || !valid {
		t.Fatalf("generated multi-unit bundle must pass quadlet dryrun: valid=%v err=%v out=%s", valid, err, out)
	}
}
