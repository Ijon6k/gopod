package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"gopod/internal/podman"
)

// GetRuntimeStatus queries real-time status of the workload from Podman or systemd.
func (d *Deployer) GetRuntimeStatus(ctx context.Context, s Service) string {
	unitName := QuadletUnitName(s)

	// 1. Quadlet Unit via systemd (binary or D-Bus user bus)
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if d.hasUserManager() {
			candidates := getQuadletCandidateUnitNames(s, unitName)
			units := d.querySystemdUnits(ctx, candidates...)
			for _, cand := range candidates {
				if st, ok := units[cand]; ok && st.LoadState == "loaded" {
					return evalSystemdState(st)
				}
			}
		}
	}

	// 2. Podman Containers — label-only matching: unlabeled containers
	// belong to other tools/users and are never counted.
	if d.podmanClient != nil {
		containers, err := d.podmanClient.GetContainers(ctx)
		if err == nil {
			var matched []podman.ContainerItem
			for _, c := range containers {
				if containerOwnedBy(c, s) {
					matched = append(matched, c)
				}
			}

			if len(matched) > 0 {
				hasRunning := false
				for _, m := range matched {
					st := strings.ToLower(m.State)
					rawSt := strings.ToLower(m.Status)
					if st == "running" || strings.HasPrefix(rawSt, "up") {
						hasRunning = true
						break
					}
				}
				if hasRunning {
					return "running"
				}
				return "stopped"
			}
		}

		// 3. Podman Pods (if runtimeTarget == 'pod' or InPod)
		pods, err := d.podmanClient.GetPods(ctx)
		if err == nil {
			podSlug := SanitizeQuadletSlug(strings.TrimPrefix(strings.TrimSpace(s.PodID), "pod_"))
			expectedPod := ""
			if podSlug != "" {
				expectedPod = fmt.Sprintf("pod_%s", podSlug)
			}
			for _, p := range pods {
				pName := p.Name
				if pName == s.ID || pName == s.Name || pName == fmt.Sprintf("pod_%s", s.ID) ||
					pName == fmt.Sprintf("pod_%s", s.Name) || pName == fmt.Sprintf("pod_%s", SanitizeQuadletSlug(s.Name)) ||
					pName == fmt.Sprintf("%s-%s", s.ProjectID, s.Name) ||
					(expectedPod != "" && pName == expectedPod) ||
					(s.PodID != "" && (pName == s.PodID || pName == fmt.Sprintf("pod_%s", s.PodID))) {
					pSt := strings.ToLower(p.Status)
					if pSt == "running" || pSt == "degraded" {
						return "running"
					}
					return "stopped"
				}
			}
		}
	}

	// 4. No systemd manager reachable and no matching container running:
	// report stopped, never "ready". A compiled unit file alone supervises
	// nothing; claiming ready while the port is dead is misinformation.
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if _, err := exec.LookPath("systemctl"); err != nil {
			return "stopped"
		}
	}

	if s.Status == "deploying" || s.Status == "building" {
		return s.Status
	}
	return "stopped"
}

// BatchRuntimeStatus queries real-time status of multiple workloads in a single pass.
func (d *Deployer) BatchRuntimeStatus(ctx context.Context, svcs []Service) map[string]string {
	result := make(map[string]string)
	if len(svcs) == 0 {
		return result
	}

	var containers []podman.ContainerItem
	var pods []podman.PodItem
	if d.podmanClient != nil {
		containers, _ = d.podmanClient.GetContainers(ctx)
		pods, _ = d.podmanClient.GetPods(ctx)
	}

	hasUserManager := d.hasUserManager()

	var allUnitCandidates []string
	serviceCandMap := make(map[string][]string)
	if hasUserManager {
		for _, s := range svcs {
			if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
				uName := QuadletUnitName(s)
				cands := getQuadletCandidateUnitNames(s, uName)
				serviceCandMap[s.ID] = cands
				allUnitCandidates = append(allUnitCandidates, cands...)
			}
		}
	}

	var systemdUnitMap map[string]SystemdUnitState
	if len(allUnitCandidates) > 0 {
		systemdUnitMap = d.querySystemdUnits(ctx, allUnitCandidates...)
	}

	for _, s := range svcs {
		// 1. Quadlet Unit
		if cands, ok := serviceCandMap[s.ID]; ok && len(systemdUnitMap) > 0 {
			matched := false
			for _, cand := range cands {
				if st, exists := systemdUnitMap[cand]; exists && st.LoadState == "loaded" {
					result[s.ID] = evalSystemdState(st)
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		} else if (s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "") && !hasUserManager {
			result[s.ID] = "stopped"
			continue
		}

		// 2. Match Containers — label-only matching: unlabeled
		// containers are never counted.
		matchedRunning := false
		matchedFound := false
		for _, c := range containers {
			if !containerOwnedBy(c, s) {
				continue
			}
			matchedFound = true
			st := strings.ToLower(c.State)
			rawSt := strings.ToLower(c.Status)
			if st == "running" || strings.HasPrefix(rawSt, "up") {
				matchedRunning = true
				break
			}
		}

		if matchedRunning {
			result[s.ID] = "running"
			continue
		}
		if matchedFound {
			result[s.ID] = "stopped"
			continue
		}

		// 3. Match Pods
		podRunning := false
		podSlug := SanitizeQuadletSlug(strings.TrimPrefix(strings.TrimSpace(s.PodID), "pod_"))
		expectedPod := ""
		if podSlug != "" {
			expectedPod = fmt.Sprintf("pod_%s", podSlug)
		}
		for _, p := range pods {
			pName := p.Name
			if pName == s.ID || pName == s.Name || pName == fmt.Sprintf("pod_%s", s.ID) ||
				pName == fmt.Sprintf("pod_%s", s.Name) || pName == fmt.Sprintf("pod_%s", SanitizeQuadletSlug(s.Name)) ||
				pName == fmt.Sprintf("%s-%s", s.ProjectID, s.Name) ||
				(expectedPod != "" && pName == expectedPod) ||
				(s.PodID != "" && (pName == s.PodID || pName == fmt.Sprintf("pod_%s", s.PodID))) {
				pSt := strings.ToLower(p.Status)
				if pSt == "running" || pSt == "degraded" {
					podRunning = true
					break
				}
			}
		}
		if podRunning {
			result[s.ID] = "running"
			continue
		}

		if s.Status == "deploying" || s.Status == "building" {
			result[s.ID] = s.Status
			continue
		}

		result[s.ID] = "stopped"
	}

	return result
}
