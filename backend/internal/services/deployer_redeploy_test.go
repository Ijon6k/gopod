package services

import (
	"strings"
	"testing"

	"gopod/internal/podman"
)

// TestSelectRedeployPlan_StartFirst verifies a healthy, labeled, port-free
// running predecessor yields start-first with a fresh collision-resistant run
// name (never reusing the live container's name).
func TestSelectRedeployPlan_StartFirst(t *testing.T) {
	s := Service{ID: "svc-a", ProjectID: "proj", Name: "web"}
	containers := []podman.ContainerItem{
		{
			ID:    "abc123def456",
			Names: []string{"/svc-a"},
			State: "running",
			Labels: map[string]string{
				"io.gopod.service": "svc-a",
				"io.gopod.project": "proj",
			},
			Ports: "",
		},
	}

	plan := selectRedeployPlan(s, containers, "svc-a", nil)
	if plan.mode != redeployStartFirst {
		t.Fatalf("expected start-first, got %q", plan.mode)
	}
	if plan.predecessorID != "abc123def456" {
		t.Fatalf("expected predecessor id recorded, got %q", plan.predecessorID)
	}
	if plan.runName == "" || plan.runName == "svc-a" {
		t.Fatalf("start-first must run under a new collision-resistant name, got %q", plan.runName)
	}
	if !strings.HasPrefix(plan.runName, "proj-web-") {
		t.Fatalf("run name should follow <project>-<service>-<hash>, got %q", plan.runName)
	}
}

// TestSelectRedeployPlan_PortConflictFallsBack verifies the classic
// stop-then-create order when the replacement needs a host port the previous
// container still holds (ports cannot be reassigned while both are alive).
func TestSelectRedeployPlan_PortConflictFallsBack(t *testing.T) {
	s := Service{ID: "svc-a", ProjectID: "proj", Name: "web", HostPort: 8080, Port: 80}
	containers := []podman.ContainerItem{
		{
			ID:     "abc123def456",
			Names:  []string{"/svc-a"},
			State:  "running",
			Labels: map[string]string{"io.gopod.service": "svc-a"},
			Ports:  "8080:80/tcp",
		},
	}

	plan := selectRedeployPlan(s, containers, "svc-a", nil)
	if plan.mode != redeployStopThenCreate {
		t.Fatalf("expected stop-then-create on host port conflict, got %q", plan.mode)
	}
	if plan.runName != "svc-a" {
		t.Fatalf("fallback must keep the canonical run name, got %q", plan.runName)
	}
}

// TestSelectRedeployPlan_NoHostPortNoConflict verifies start-first is allowed
// when only the container port is set (internal-only service).
func TestSelectRedeployPlan_NoHostPortNoConflict(t *testing.T) {
	s := Service{ID: "svc-a", ProjectID: "proj", Name: "web", HostPort: 0, Port: 8080}
	containers := []podman.ContainerItem{
		{
			ID:     "abc123def456",
			Names:  []string{"/svc-a"},
			State:  "running",
			Labels: map[string]string{"io.gopod.service": "svc-a"},
			Ports:  "8080/tcp",
		},
	}

	plan := selectRedeployPlan(s, containers, "svc-a", nil)
	if plan.mode != redeployStartFirst {
		t.Fatalf("expected start-first when no host ports are involved, got %q", plan.mode)
	}
}

// TestSelectRedeployPlan_UnlabeledSiblingIgnored verifies a RUNNING container
// that merely shares the service name (another project, or a user-created
// container) never becomes the predecessor: only the ownership label counts.
func TestSelectRedeployPlan_UnlabeledSiblingIgnored(t *testing.T) {
	s := Service{ID: "svc-a", ProjectID: "proj", Name: "web"}
	containers := []podman.ContainerItem{
		{
			ID:     "zzz123def456",
			Names:  []string{"/web"},
			State:  "running",
			Labels: nil, // user-created container: gopod must not touch it
		},
	}

	plan := selectRedeployPlan(s, containers, "svc-a", nil)
	if plan.mode != redeployStopThenCreate {
		t.Fatalf("unlabeled container must not be claimed as predecessor, got %q", plan.mode)
	}
}

// TestSelectRedeployPlan_PausedPredecessor verifies paused containers still
// count as live predecessors (their netns keeps the bindings).
func TestSelectRedeployPlan_PausedPredecessor(t *testing.T) {
	s := Service{ID: "svc-a", ProjectID: "proj", Name: "web"}
	containers := []podman.ContainerItem{
		{
			ID:     "abc123def456",
			Names:  []string{"/svc-a"},
			State:  "paused",
			Labels: map[string]string{"io.gopod.service": "svc-a"},
		},
	}

	plan := selectRedeployPlan(s, containers, "svc-a", nil)
	if plan.mode != redeployStartFirst {
		t.Fatalf("expected start-first with paused predecessor, got %q", plan.mode)
	}
}

// TestSelectRedeployPlan_PodMemberNeverStartFirst verifies pod members keep
// the previous stop-then-create behavior (start-first applies to plain
// containers only, since pod members are replaced inside their pod).
func TestSelectRedeployPlan_PodMemberNeverStartFirst(t *testing.T) {
	s := Service{ID: "svc-a", ProjectID: "proj", Name: "web", InPod: true}
	if plan := selectRedeployPlan(s, nil, "svc-a", nil); plan.mode != redeployStopThenCreate {
		t.Fatalf("pod member must not go start-first, got %q", plan.mode)
	}
	s2 := Service{ID: "svc-a", RuntimeTarget: "pod"}
	if plan := selectRedeployPlan(s2, nil, "svc-a", nil); plan.mode != redeployStopThenCreate {
		t.Fatalf("runtimeTarget pod must not go start-first, got %q", plan.mode)
	}
}

// TestContainerBindsHostPort verifies the port-format matching used for
// conflict detection across listing variants.
func TestContainerBindsHostPort(t *testing.T) {
	cases := []struct {
		ports    string
		hostPort int
		want     bool
	}{
		{"8080:80/tcp", 8080, true},
		{"8080:80/tcp", 9999, false},
		{"8080:80/tcp, 9090:90/tcp", 9090, true},
		{"8080:80/tcp, 9090:90/tcp", 880, false}, // no suffix/substring false positive
		{"", 8080, false},
		{"8080/tcp", 8080, false}, // container-only port: no host binding
	}
	for _, tc := range cases {
		c := podman.ContainerItem{Ports: tc.ports}
		if got := containerBindsHostPort(c, tc.hostPort); got != tc.want {
			t.Fatalf("containerBindsHostPort(%q, %d) = %v, want %v", tc.ports, tc.hostPort, got, tc.want)
		}
	}
}
