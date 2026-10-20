package services

import (
	"context"
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
	}, nil)

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
	content := d.generateDefaultQuadletContent(context.Background(), s, "stack")
	files := SplitQuadletUnits(content, "stack")
	if len(files) == 0 {
		t.Fatalf("generated content produced no unit files:\n%s", content)
	}
	valid, out, err := ValidateQuadletFiles(files)
	if err != nil || !valid {
		t.Fatalf("generated multi-unit bundle must pass quadlet dryrun: valid=%v err=%v out=%s", valid, err, out)
	}
}

func TestPodBundleEmitsOneContainerPerChild(t *testing.T) {
	pod := Service{ID: "proj-pod-1", Name: "api-stack", ProjectID: "proj-pod", Type: "pod"}
	members := []Service{
		{
			ID:            "proj-api-1",
			Name:          "api",
			ProjectID:     "proj-pod",
			InPod:         true,
			PodID:         pod.ID,
			Image:         "docker.io/library/nginx:alpine",
			RestartPolicy: "unless-stopped",
			EnvVars:       []EnvVar{{Key: "API_MODE", Value: "edge"}},
		},
		func() Service {
			m := Service{
				ID:        "proj-db-9",
				Name:      "db",
				ProjectID: "proj-pod",
				InPod:     true,
				PodID:     pod.ID,
				Image:     "docker.io/library/postgres:16",
			}
			m.Advanced = &ServiceAdvancedConfig{}
			m.Advanced.Storage.Volumes = []VolumeMountConfig{
				{Source: "db_data", Target: "/var/lib/postgresql/data"},
			}
			return m
		}(),
	}

	bundle := buildPodQuadletBundle(pod, "pod_proj-pod-1", []string{"18401:80"}, members)
	files := SplitQuadletUnits(bundle, "api-stack")

	if len(files) != 3 {
		t.Fatalf("bundle must contain the .pod plus one .container per member, got %d files: %v", len(files), files)
	}
	var podFile, apiFile, dbFile string
	for name, content := range files {
		switch {
		case strings.HasSuffix(name, ".pod"):
			podFile = content
		case strings.Contains(content, "ContainerName=proj-api-1"):
			apiFile = content
		case strings.Contains(content, "ContainerName=proj-db-9"):
			dbFile = content
		}
	}
	if podFile == "" || apiFile == "" || dbFile == "" {
		t.Fatalf("missing expected unit:\n%s", bundle)
	}

	// Pod unit: publishes ports, pod-scoped labels.
	if !strings.Contains(podFile, "PublishPort=18401:80") {
		t.Errorf(".pod must carry PublishPort:\n%s", podFile)
	}
	if !strings.Contains(podFile, "Label=io.gopod.service=proj-pod-1") {
		t.Errorf(".pod must carry the ownership label:\n%s", podFile)
	}

	// Member units: Pod= reference, env + volumes from the child, no ports.
	for name, file := range map[string]string{"api": apiFile, "db": dbFile} {
		if !strings.Contains(file, "Pod=pod_proj-pod-1.pod") {
			t.Errorf("%s member must reference Pod=:\n%s", name, file)
		}
		if strings.Contains(file, "PublishPort=") {
			t.Errorf("%s member must not publish ports (pod does):\n%s", name, file)
		}
		if !strings.Contains(file, "Label=io.gopod.service=") {
			t.Errorf("%s member must carry an ownership label:\n%s", name, file)
		}
	}
	if !strings.Contains(apiFile, `Environment="API_MODE=edge"`) {
		t.Errorf("api member must carry its env:\n%s", apiFile)
	}
	if !strings.Contains(dbFile, "Volume=db") || !strings.Contains(dbFile, "/var/lib/postgresql/data:Z") {
		t.Errorf("db member must carry its advanced volume:\n%s", dbFile)
	}

	if valid, out, err := ValidateQuadletFiles(files); err != nil || !valid {
		t.Fatalf("full pod bundle must pass quadlet dryrun: valid=%v err=%v out=%s", valid, err, out)
	}
}
