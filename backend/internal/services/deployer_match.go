package services

import (
	"context"
	"errors"
	"strings"

	"gopod/internal/podman"
)

// Ownership matching for gopod-managed runtime objects: strictly label-based.
//
// gopod attaches `io.gopod.service` / `io.gopod.project` at every write path
// (RunContainer, CreatePod, auto-pods, quadlet units, kube manifests), so
// matching is strictly label-based. Two similarly-named services — across or
// within projects — can therefore never delete, stop, or misreport each
// other's containers, pods, or volumes. Container has no label → it is not
// ours and must never be touched.

// containerOwnedBy reports whether c belongs to service s per strict
// ownership rules. Unlabeled containers are NEVER owned.
func containerOwnedBy(c podman.ContainerItem, s Service) bool {
	if c.Labels == nil {
		return false
	}
	// 1. Primary: gopod service ownership label.
	if c.Labels["io.gopod.service"] == s.ID {
		return true
	}
	// 2. Documented exception: compose-managed stacks keep matching when the
	//    compose project label equals the service ID (compose deploys set it).
	if c.Labels["io.podman.compose.project"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID {
		return true
	}
	// 3. Quadlet transient units match their exact generated unit name only.
	for _, cand := range quadletUnitLabelsFor(s) {
		if c.Labels["io.podman.quadlet.unit"] == cand {
			return true
		}
	}
	return false
}

// quadletUnitLabelsFor returns the exact io.podman.quadlet.unit values (e.g.
// "<unit>.container", "<unit>.pod") that belong to service s.
func quadletUnitLabelsFor(s Service) []string {
	var out []string
	for _, cand := range getQuadletCandidateUnitNames(s, QuadletUnitName(s)) {
		baseUnit := strings.TrimSuffix(cand, ".service")
		if baseUnit == "" {
			continue
		}
		out = append(out, baseUnit+".container", baseUnit+".pod")
	}
	return out
}

// podOwnedBy reports whether p belongs to service s per strict ownership
// rules. Cross-project pods are never owned.
func podOwnedBy(p podman.PodItem, s Service) bool {
	return p.Labels != nil && p.Labels["io.gopod.service"] == s.ID
}

// podLegacyCandidateOwned matches pods from pre-canonical naming schemes
// (legacyPodCandidates) only when they provably belong to gopod: either the
// service label matches or the pod carries this service's project label.
// Two projects with same-named services can therefore never garbage-collect
// each other's pods.
func podLegacyCandidateOwned(p podman.PodItem, s Service) bool {
	if p.Labels == nil {
		return false
	}
	if p.Labels["io.gopod.service"] == s.ID {
		return true
	}
	return p.Labels["io.gopod.project"] == s.ProjectID
}

// volumeOwnedBy reports whether a named volume belongs to service s.
func volumeOwnedBy(v podman.VolumeItem, s Service) bool {
	if v.Labels == nil {
		return false
	}
	return v.Labels["io.gopod.service"] == s.ID ||
		v.Labels["io.podman.compose.project"] == s.ID ||
		v.Labels["com.docker.compose.project"] == s.ID
}

// ownedContainersByName returns the current containers of service s keyed by
// their exact runtime names (leading slash stripped) and container IDs. Used wherever a
// unit-identity derived NAME or ID must be resolved to an actual container before a
// destructive operation (never delete by name alone).
func (d *Deployer) ownedContainersByName(ctx context.Context, s Service) (map[string]podman.ContainerItem, error) {
	if d.podmanClient == nil {
		return nil, errors.New("no podman client")
	}
	containers, err := d.podmanClient.GetContainers(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]podman.ContainerItem, len(containers)*2)
	for _, c := range containers {
		if c.ID != "" {
			out[c.ID] = c
		}
		for _, n := range c.Names {
			out[strings.TrimPrefix(n, "/")] = c
		}
	}
	return out, nil
}
