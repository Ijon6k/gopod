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
	// 1. Quadlet Unit
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", fmt.Sprintf("%s.service", s.Name))
			out, err := cmd.CombinedOutput()
			st := strings.TrimSpace(string(out))
			if err == nil && st == "active" {
				return "running"
			}
			if st == "failed" {
				return "failed"
			}
			if st == "activating" {
				return "deploying"
			}
			return "stopped"
		}
	}

	// 2. Podman Containers
	if d.podmanClient != nil {
		containers, err := d.podmanClient.GetContainers(ctx)
		if err == nil {
			var matched []podman.ContainerItem
			for _, c := range containers {
				isMatch := false
				if c.Labels != nil {
					if c.Labels["io.gopod.service"] == s.ID ||
						c.Labels["com.docker.compose.project"] == s.ID ||
						c.Labels["io.podman.compose.project"] == s.ID ||
						c.Labels["com.docker.compose.project"] == s.Name {
						isMatch = true
					}
				}
				if !isMatch && len(c.Names) > 0 {
					firstName := strings.TrimPrefix(c.Names[0], "/")
					if firstName == s.ID ||
						firstName == s.Name ||
						strings.HasPrefix(firstName, s.ID) ||
						strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) ||
						strings.HasPrefix(firstName, fmt.Sprintf("pod_%s", s.ID)) {
						isMatch = true
					}
				}
				if isMatch {
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
			for _, p := range pods {
				pName := p.Name
				if pName == s.ID || pName == s.Name || pName == fmt.Sprintf("pod_%s", s.ID) || pName == fmt.Sprintf("%s-%s", s.ProjectID, s.Name) {
					pSt := strings.ToLower(p.Status)
					if pSt == "running" || pSt == "degraded" {
						return "running"
					}
					return "stopped"
				}
			}
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

	hasSystemctl := false
	if _, err := exec.LookPath("systemctl"); err == nil {
		hasSystemctl = true
	}

	for _, s := range svcs {
		if s.Status == "deploying" || s.Status == "building" {
			result[s.ID] = s.Status
			continue
		}

		// 1. Quadlet Unit
		if (s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "") && hasSystemctl {
			cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", fmt.Sprintf("%s.service", s.Name))
			out, err := cmd.CombinedOutput()
			st := strings.TrimSpace(string(out))
			if err == nil && st == "active" {
				result[s.ID] = "running"
				continue
			}
			if st == "failed" {
				result[s.ID] = "failed"
				continue
			}
			if st == "activating" {
				result[s.ID] = "deploying"
				continue
			}
			result[s.ID] = "stopped"
			continue
		}

		// 2. Match Containers
		matchedRunning := false
		matchedFound := false
		for _, c := range containers {
			isMatch := false
			if c.Labels != nil {
				if c.Labels["io.gopod.service"] == s.ID ||
					c.Labels["com.docker.compose.project"] == s.ID ||
					c.Labels["io.podman.compose.project"] == s.ID ||
					c.Labels["com.docker.compose.project"] == s.Name {
					isMatch = true
				}
			}
			if !isMatch && len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if firstName == s.ID ||
					firstName == s.Name ||
					strings.HasPrefix(firstName, s.ID) ||
					strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) ||
					strings.HasPrefix(firstName, fmt.Sprintf("pod_%s", s.ID)) {
					isMatch = true
				}
			}
			if isMatch {
				matchedFound = true
				st := strings.ToLower(c.State)
				rawSt := strings.ToLower(c.Status)
				if st == "running" || strings.HasPrefix(rawSt, "up") {
					matchedRunning = true
					break
				}
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
		for _, p := range pods {
			pName := p.Name
			if pName == s.ID || pName == s.Name || pName == fmt.Sprintf("pod_%s", s.ID) || pName == fmt.Sprintf("%s-%s", s.ProjectID, s.Name) {
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

		result[s.ID] = "stopped"
	}

	return result
}
