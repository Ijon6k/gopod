package services

import (
	"strings"
	"testing"

	"gopod/internal/podman"
)

// Collision pair: two services whose names
// are prefixes (and substrings) of one another, plus a same-named service in
// another project. All destructive ops must match ONLY the owning labels and
// never resolve by name similarity.
func collideFixture() (Service, []podman.ContainerItem, []podman.PodItem, []podman.VolumeItem) {
	web := Service{ID: "svc-web", ProjectID: "proj-a", Name: "web"}
	containers := []podman.ContainerItem{
		// Ours: gopod-labeled container of service A.
		{
			ID:     "aaaaamain",
			Names:  []string{"/svc-web"},
			State:  "running",
			Labels: map[string]string{"io.gopod.service": "svc-web", "io.gopod.project": "proj-a"},
		},
		// Service B (web-history): has svc id "svc-web-history" and CONTAINS
		// the string "svc-web" in every name. Must NOT be owned by A.
		{
			ID:     "bbbbbbhist",
			Names:  []string{"/svc-web-history"},
			State:  "running",
			Labels: map[string]string{"io.gopod.service": "svc-web-history", "io.gopod.project": "proj-a"},
		},
		// Unlabeled user container that happens to be named "web".
		{
			ID:     "ccccccuser",
			Names:  []string{"/web"},
			State:  "running",
			Labels: nil,
		},
		// Compose member with a DIFFERENT service id must not be claimed by A.
		{
			ID:     "ddddddcomp",
			Names:  []string{"/other-stack-web"},
			State:  "running",
			Labels: map[string]string{"com.docker.compose.project": "proj-b-other"},
		},
	}
	pods := []podman.PodItem{
		{
			Name:   "pod_svc-web",
			Labels: map[string]string{"io.gopod.service": "svc-web", "io.gopod.project": "proj-a"},
		},
		{
			// Same-named pod in ANOTHER project, unlabeled: must never match.
			Name:   "pod_svc-web",
			Labels: nil,
		},
		{
			// Legacy-scheme pod of a different project: io.gopod.project mismatch.
			Name:   "pod_web-history",
			Labels: map[string]string{"io.gopod.project": "proj-b", "io.gopod.service": "svc-web-history"},
		},
	}
	volumes := []podman.VolumeItem{
		{Name: "web-data-ours", Labels: map[string]string{"io.gopod.service": "svc-web"}},
		{Name: "web-data-sibling", Labels: map[string]string{"io.gopod.service": "svc-web-history"}},
		{Name: "unlabeled-random-volume", Labels: nil},
	}
	return web, containers, pods, volumes
}

// TestContainerOwnedBy_PrefixCollisionIsolation verifies destructive matching
// never resolves by name similarity: only strict label ownership counts.
func TestContainerOwnedBy_PrefixCollisionIsolation(t *testing.T) {
	web, containers, _, _ := collideFixture()

	owned := 0
	for _, c := range containers {
		if containerOwnedBy(c, web) {
			owned++
			if c.ID != "aaaaamain" {
				t.Fatalf("container %q (%s) must not be owned by service %q", c.ID, c.Names[0], web.ID)
			}
		}
	}
	if owned != 1 {
		t.Fatalf("expected exactly 1 owned container, got %d", owned)
	}
}

// TestContainerOwnedBy_UnlabeledNeverOwned documents the contract: gopod
// never touches unlabeled containers even when their names match exactly.
func TestContainerOwnedBy_UnlabeledNeverOwned(t *testing.T) {
	web, _, _, _ := collideFixture()
	unlabeled := podman.ContainerItem{ID: "ccccccuser", Names: []string{"/web"}, Labels: nil}
	if containerOwnedBy(unlabeled, web) {
		t.Fatal("unlabeled container must never be owned even by exact name")
	}
}

// TestContainerOwnedBy_ComposeProjectLabelException verifies the documented
// compose exception only matches the EXACT service id (not substrings).
func TestContainerOwnedBy_ComposeProjectLabelException(t *testing.T) {
	s := Service{ID: "svc-cool", ProjectID: "proj", Name: "cool"}
	own := podman.ContainerItem{Labels: map[string]string{"com.docker.compose.project": "svc-cool"}}
	notOwn := podman.ContainerItem{Labels: map[string]string{"com.docker.compose.project": "svc-cool-family"}}
	if !containerOwnedBy(own, s) {
		t.Fatal("exact compose project label must match")
	}
	if containerOwnedBy(notOwn, s) {
		t.Fatal("compose project label must match only on exact equality")
	}
}

// TestPodOwnedBy_CrossProjectSameNameIsolation verifies pods are matched by
// label only; a same-named unlabeled pod in another project is untouchable.
func TestPodOwnedBy_CrossProjectSameNameIsolation(t *testing.T) {
	web, _, pods, _ := collideFixture()

	hitOther := false
	for _, p := range pods {
		if podOwnedBy(p, web) {
			if p.Name == "pod_svc-web" && p.Labels != nil && p.Labels["io.gopod.service"] == "svc-web" {
				continue // genuinely ours
			}
			t.Fatalf("pod %q must not be owned by %q", p.Name, web.ID)
		}
		if p.Name == "pod_svc-web" && p.Labels == nil {
			hitOther = true
		}
	}
	if !hitOther {
		t.Fatal("fixture must contain the cross-project same-name pod")
	}
}

// TestPodLegacyCandidateOwned verifies legacy-scheme sweeping only removes
// pods that carry the same project label: cross-project same-name pods can
// never be garbage-collected.
func TestPodLegacyCandidateOwned(t *testing.T) {
	ours := Service{ID: "svc-web", ProjectID: "proj-a", Name: "web"}

	if !podLegacyCandidateOwned(podman.PodItem{Name: "pod_web", Labels: map[string]string{"io.gopod.project": "proj-a"}}, ours) {
		t.Fatal("legacy pod of the same project must be owned")
	}
	if podLegacyCandidateOwned(podman.PodItem{Name: "pod_web", Labels: map[string]string{"io.gopod.project": "proj-b"}}, ours) {
		t.Fatal("legacy pod of another project must never be owned")
	}
	if podLegacyCandidateOwned(podman.PodItem{Name: "pod_web", Labels: nil}, ours) {
		t.Fatal("unlabeled legacy pod must never be owned")
	}
	if !podLegacyCandidateOwned(podman.PodItem{Name: "pod_web", Labels: map[string]string{"io.gopod.service": "svc-web"}}, ours) {
		t.Fatal("pod directly labeled with the service id must be owned")
	}
}

// TestVolumeOwnedBy_PrefixCollisionIsolation verifies volumes are matched by
// labels, not by bare name prefixes (web vs web2 hazard).
func TestVolumeOwnedBy_PrefixCollisionIsolation(t *testing.T) {
	web, _, _, volumes := collideFixture()

	owned := 0
	for _, v := range volumes {
		if volumeOwnedBy(v, web) {
			owned++
			if v.Name != "web-data-ours" {
				t.Fatalf("volume %q must not be owned by %q", v.Name, web.ID)
			}
		}
	}
	if owned != 1 {
		t.Fatalf("expected exactly 1 owned volume, got %d", owned)
	}
	if volumeOwnedBy(podman.VolumeItem{Name: "svc-web2-data", Labels: nil}, web) {
		t.Fatal("prefix-similar unlabeled volume must never be owned")
	}
}

// TestQuadletUnitLabelsFor verifies quadlet unit-label matching is anchored to
// exact generated unit names only.
func TestQuadletUnitLabelsFor(t *testing.T) {
	s := Service{ID: "svc-quad", Name: "myapp", QuadletConfig: "[Container]\nImage=nginx\n"}
	labels := quadletUnitLabelsFor(s)
	if len(labels) == 0 {
		t.Fatal("expected at least one quadlet unit label candidate")
	}
	for _, l := range labels {
		if l != "" && !strings.HasSuffix(l, ".container") && !strings.HasSuffix(l, ".pod") {
			t.Fatalf("unit label candidates must end with .container/.pod, got %q", l)
		}
	}
	c := podman.ContainerItem{Labels: map[string]string{"io.podman.quadlet.unit": labels[0]}}
	if !containerOwnedBy(c, s) {
		t.Fatal("container carrying the exact quadlet unit label must be owned")
	}
	c.Labels["io.podman.quadlet.unit"] = labels[0] + "-other.container"
	if containerOwnedBy(c, s) {
		t.Fatal("different quadlet unit label must not match")
	}
}
