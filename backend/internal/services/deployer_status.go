package services

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"gopod/internal/podman"
)

// SystemdUnitState captures systemctl show status properties.
type SystemdUnitState struct {
	ID          string
	LoadState   string
	ActiveState string
	SubState    string
	Result      string
	NRestarts   int
}

func parseSystemdShowOutput(output string) map[string]SystemdUnitState {
	units := make(map[string]SystemdUnitState)
	blocks := strings.Split(output, "\n\n")
	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		var st SystemdUnitState
		for _, line := range lines {
			parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
			if len(parts) != 2 {
				continue
			}
			k, v := parts[0], parts[1]
			switch k {
			case "Id":
				st.ID = v
			case "LoadState":
				st.LoadState = v
			case "ActiveState":
				st.ActiveState = v
			case "SubState":
				st.SubState = v
			case "Result":
				st.Result = v
			case "NRestarts":
				st.NRestarts, _ = strconv.Atoi(v)
			}
		}
		if st.ID != "" {
			units[st.ID] = st
		}
	}
	return units
}

func querySystemdUnits(ctx context.Context, unitNames ...string) map[string]SystemdUnitState {
	if len(unitNames) == 0 {
		return nil
	}
	args := append([]string{"--user", "show"}, unitNames...)
	args = append(args, "--property=Id,LoadState,ActiveState,SubState,Result,NRestarts")
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil
	}
	return parseSystemdShowOutput(string(out))
}

func evalSystemdState(st SystemdUnitState) string {
	if st.LoadState != "loaded" {
		return "stopped"
	}
	if st.ActiveState == "activating" {
		if st.SubState == "auto-restart" || st.NRestarts > 3 || st.Result == "start-limit-hit" || st.Result == "exit-code" {
			return "failed"
		}
		return "deploying"
	}
	if st.ActiveState == "failed" {
		return "failed"
	}
	if st.ActiveState == "active" {
		if st.SubState == "running" || st.SubState == "active" {
			return "running"
		}
		return "running"
	}
	if st.ActiveState == "inactive" || st.ActiveState == "deactivating" {
		return "stopped"
	}
	return "stopped"
}

func getQuadletCandidateUnitNames(s Service, unitName string) []string {
	if s.Type == "pod" || s.RuntimeTarget == "pod" || strings.Contains(s.QuadletConfig, "[Pod]") {
		return []string{
			fmt.Sprintf("%s-pod.service", unitName),
			fmt.Sprintf("pod_%s-pod.service", unitName),
			fmt.Sprintf("%s.service", unitName),
		}
	}
	return []string{
		fmt.Sprintf("%s.service", unitName),
		fmt.Sprintf("%s-pod.service", unitName),
	}
}

// GetRuntimeStatus queries real-time status of the workload from Podman or systemd.
func (d *Deployer) GetRuntimeStatus(ctx context.Context, s Service) string {
	unitName := QuadletUnitName(s)

	// 1. Quadlet Unit via systemd
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			candidates := getQuadletCandidateUnitNames(s, unitName)
			units := querySystemdUnits(ctx, candidates...)
			for _, cand := range candidates {
				if st, ok := units[cand]; ok && st.LoadState == "loaded" {
					return evalSystemdState(st)
				}
			}
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
						firstName == unitName ||
						firstName == fmt.Sprintf("systemd-%s", unitName) ||
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

	var allUnitCandidates []string
	serviceCandMap := make(map[string][]string)
	if hasSystemctl {
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
		systemdUnitMap = querySystemdUnits(ctx, allUnitCandidates...)
	}

	for _, s := range svcs {
		if s.Status == "deploying" || s.Status == "building" {
			result[s.ID] = s.Status
			continue
		}

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
		}

		// 2. Match Containers
		matchedRunning := false
		matchedFound := false
		unitName := QuadletUnitName(s)
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
					firstName == unitName ||
					firstName == fmt.Sprintf("systemd-%s", unitName) ||
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
