package services

import (
	"context"
	"strings"
	"testing"
)

// The generated [Pod] unit must not carry an InfraImage= key: an unknown key
// aborts quadlet conversion of the whole unit, and pinning registry.k8s.io/
// pause would force a network pull on start. podman's builtin infra image is
// used implicitly instead.
func TestQuadletPodUnitsDoNotPinInfraImage(t *testing.T) {
	s := Service{ID: "svc-infra-1", ProjectID: "proj", Name: "web"}
	bundle := buildPodQuadletBundle(s, "pod_svc-infra-1", []string{"18401:80"}, nil)
	if strings.Contains(bundle, "InfraImage=") {
		t.Fatalf("generated .pod unit pins InfraImage=; podman must manage the pause image:\n%s", bundle)
	}

	d := NewDeployer(nil, nil, nil, t.TempDir())
	multi := d.generateDefaultQuadletContent(context.Background(), Service{
		ID:          "svc-infra-2",
		ProjectID:   "proj",
		Name:        "stack",
		ComposeYaml: "services:\n  web:\n    image: nginx:alpine\n  db:\n    image: postgres:16\n",
	}, "stack")
	if strings.Contains(multi, "InfraImage=") {
		t.Fatalf("generated multi-service bundle pins InfraImage=:\n%s", multi)
	}
}
