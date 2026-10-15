package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

func (d *Deployer) querySystemdUnits(ctx context.Context, unitNames ...string) map[string]SystemdUnitState {
	if len(unitNames) == 0 {
		return nil
	}
	// Prefer the native binary; fall back to the D-Bus user bus when the
	// engine runs containerized with the host bus mounted.
	if _, err := exec.LookPath("systemctl"); err != nil {
		return querySystemdUnitsViaBus(ctx, unitNames...)
	}
	args := append([]string{"--user", "show"}, unitNames...)
	args = append(args, "--property=Id,LoadState,ActiveState,SubState,Result,NRestarts")
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	d.setupCmdEnv(cmd)
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
	seen := make(map[string]bool)
	var list []string

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		isPod := strings.HasSuffix(name, "-pod.service") || strings.HasSuffix(name, "-pod")
		rawBase := strings.TrimSuffix(name, ".service")
		if isPod {
			rawBase = strings.TrimSuffix(rawBase, "-pod")
		}
		cleanBase := SanitizeQuadletSlug(rawBase)
		if cleanBase == "" {
			return
		}
		var finalName string
		if isPod {
			finalName = fmt.Sprintf("%s-pod.service", cleanBase)
		} else {
			finalName = fmt.Sprintf("%s.service", cleanBase)
		}
		if !seen[finalName] {
			seen[finalName] = true
			list = append(list, finalName)
		}
	}

	// 1. From SplitQuadletUnits and parsed AST directives
	if strings.TrimSpace(s.QuadletConfig) != "" {
		cleanDefault := unitName
		if cleanDefault == "" {
			cleanDefault = QuadletUnitName(s)
		}
		units := SplitQuadletUnits(s.QuadletConfig, cleanDefault)
		for fname := range units {
			if strings.HasSuffix(fname, ".pod") {
				base := strings.TrimSuffix(fname, ".pod")
				add(fmt.Sprintf("%s-pod.service", base))
				add(fmt.Sprintf("pod_%s-pod.service", base))
			} else if strings.HasSuffix(fname, ".container") {
				base := strings.TrimSuffix(fname, ".container")
				add(fmt.Sprintf("%s.service", base))
			}
		}
		parsed := ParseQuadlet(s.QuadletConfig)
		if parsed.ContainerName != "" {
			add(parsed.ContainerName)
		}
		if parsed.Pod != "" {
			cleanPod := strings.TrimSuffix(parsed.Pod, ".pod")
			add(fmt.Sprintf("%s-pod.service", cleanPod))
			add(fmt.Sprintf("pod_%s-pod.service", cleanPod))
		}
	}

	// 2. From standard unit name derivations
	// Extract all services and container_name definitions from compose YAML
	if s.ComposeYaml != "" {
		for _, name := range ExtractAllComposeWorkloadNames(s.ComposeYaml) {
			add(name)
		}
	}

	cleanName := QuadletUnitName(s)
	add(cleanName)
	add(fmt.Sprintf("%s-pod.service", cleanName))
	add(fmt.Sprintf("pod_%s-pod.service", cleanName))

	if unitName != "" && unitName != cleanName {
		add(unitName)
		add(fmt.Sprintf("%s-pod.service", unitName))
		add(fmt.Sprintf("pod_%s-pod.service", unitName))
	}

	if s.Name != "" {
		add(QuadletUnitName(Service{Name: s.Name}))
		add(s.Name)
	}

	if s.ID != "" {
		add(s.ID)
	}

	// 3. Scan Quadlet directories on filesystem for units referencing this service
	dirs := []string{"./data/quadlets"}
	if envDir := os.Getenv("QUADLET_DIR"); envDir != "" {
		dirs = append(dirs, envDir)
	} else if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".config", "containers", "systemd"))
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fname := entry.Name()
			ext := filepath.Ext(fname)
			if ext != ".container" && ext != ".pod" {
				continue
			}
			stem := strings.TrimSuffix(fname, ext)
			fullPath := filepath.Join(dir, fname)
			isMatch := false
			cleanPrefix := fmt.Sprintf("%s-%s", s.ProjectID, s.Name)
			if stem == cleanName || stem == s.Name || stem == s.ID || strings.HasPrefix(stem, s.ID) || (cleanPrefix != "" && strings.HasPrefix(stem, cleanPrefix)) {
				isMatch = true
			} else if data, err := os.ReadFile(fullPath); err == nil {
				dataStr := string(data)
				if (s.ID != "" && strings.Contains(dataStr, "io.gopod.service="+s.ID)) ||
					(cleanPrefix != "" && strings.Contains(dataStr, "io.gopod.service="+cleanPrefix+"-")) ||
					(cleanPrefix != "" && strings.Contains(dataStr, "io.gopod.service="+cleanPrefix)) ||
					(s.Name != "" && strings.Contains(dataStr, "Label=io.gopod.service="+s.Name)) ||
					(s.ProjectID != "" && s.Name != "" && strings.Contains(dataStr, fmt.Sprintf("io.gopod.project=%s", s.ProjectID)) && strings.Contains(dataStr, s.Name)) {
					isMatch = true
				}
				if cName := extractDirective(dataStr, "ContainerName"); cName != "" && (cName == cleanName || cName == s.Name || (s.ID != "" && strings.HasPrefix(cName, s.ID))) {
					isMatch = true
				}
			}
			if isMatch {
				if ext == ".pod" {
					add(fmt.Sprintf("%s-pod.service", stem))
				} else {
					add(fmt.Sprintf("%s.service", stem))
				}
			}
		}
	}

	return list
}

// findServiceUnitFilesAndCandidates thoroughly discovers all potential systemd unit names,
// container names, and unit file paths associated with a service for zero-orphan teardown.
func (d *Deployer) findServiceUnitFilesAndCandidates(s Service) (candidateUnits []string, candidateContainers []string, filesToDelete []string) {
	seenUnits := make(map[string]bool)
	seenContainers := make(map[string]bool)
	seenFiles := make(map[string]bool)

	addUnit := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		isPod := strings.HasSuffix(name, "-pod.service") || strings.HasSuffix(name, "-pod")
		rawBase := strings.TrimSuffix(name, ".service")
		if isPod {
			rawBase = strings.TrimSuffix(rawBase, "-pod")
		}
		cleanBase := SanitizeQuadletSlug(rawBase)
		if cleanBase == "" {
			return
		}

		var finalName string
		if isPod {
			finalName = fmt.Sprintf("%s-pod.service", cleanBase)
		} else {
			finalName = fmt.Sprintf("%s.service", cleanBase)
		}
		if !seenUnits[finalName] {
			seenUnits[finalName] = true
			candidateUnits = append(candidateUnits, finalName)
		}
		if !seenContainers[cleanBase] {
			seenContainers[cleanBase] = true
			candidateContainers = append(candidateContainers, cleanBase)
		}
	}

	addContainer := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		clean := strings.TrimPrefix(name, "/")
		if clean == "" {
			return
		}
		if !seenContainers[clean] {
			seenContainers[clean] = true
			candidateContainers = append(candidateContainers, clean)
		}
		slug := SanitizeQuadletSlug(clean)
		if slug != "" && !seenContainers[slug] {
			seenContainers[slug] = true
			candidateContainers = append(candidateContainers, slug)
		}
	}

	addFile := func(path string) {
		if path == "" || seenFiles[path] {
			return
		}
		seenFiles[path] = true
		filesToDelete = append(filesToDelete, path)
	}

	// 1. Base names
	cleanName := QuadletUnitName(s)
	cleanPrefix := fmt.Sprintf("%s-%s", s.ProjectID, s.Name)
	addContainer(s.ID)
	addContainer(s.Name)
	addContainer(cleanName)
	addContainer(cleanPrefix)
	addContainer(fmt.Sprintf("systemd-%s", cleanName))
	addUnit(s.ID)
	addUnit(s.Name)
	addUnit(cleanName)

	// 2. Parsed Quadlet
	if strings.TrimSpace(s.QuadletConfig) != "" {
		parsed := ParseQuadlet(s.QuadletConfig)
		if parsed.ContainerName != "" {
			addContainer(parsed.ContainerName)
			addUnit(parsed.ContainerName)
		}
		if parsed.Pod != "" {
			addContainer(parsed.Pod)
			addUnit(fmt.Sprintf("%s-pod.service", parsed.Pod))
		}
		units := SplitQuadletUnits(s.QuadletConfig, cleanName)
		for fname := range units {
			ext := filepath.Ext(fname)
			stem := strings.TrimSuffix(fname, ext)
			addContainer(stem)
			if ext == ".pod" {
				addUnit(fmt.Sprintf("%s-pod.service", stem))
			} else {
				addUnit(fmt.Sprintf("%s.service", stem))
			}
		}
	}

	// 3. Parsed Compose YAML
	if s.ComposeYaml != "" {
		for _, name := range ExtractAllComposeWorkloadNames(s.ComposeYaml) {
			addContainer(name)
			addUnit(name)
		}
	}

	// 4. File system discovery
	searchDirs := []string{
		filepath.Join(d.dataDir, "quadlets"),
	}
	systemdQuadletDir := os.Getenv("QUADLET_DIR")
	if systemdQuadletDir == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			systemdQuadletDir = filepath.Join(home, ".config", "containers", "systemd")
		}
	}
	if systemdQuadletDir != "" {
		searchDirs = append(searchDirs, systemdQuadletDir)
	}

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fname := entry.Name()
			ext := filepath.Ext(fname)
			if ext != ".container" && ext != ".pod" && ext != ".volume" && ext != ".network" {
				continue
			}
			stem := strings.TrimSuffix(fname, ext)
			fullPath := filepath.Join(dir, fname)

			isMatch := false
			if seenContainers[stem] || seenContainers[SanitizeQuadletSlug(stem)] || seenUnits[fmt.Sprintf("%s.service", stem)] || seenUnits[fmt.Sprintf("%s-pod.service", stem)] {
				isMatch = true
			} else if data, err := os.ReadFile(fullPath); err == nil {
				dataStr := string(data)
				if (s.ID != "" && strings.Contains(dataStr, "io.gopod.service="+s.ID)) ||
					(cleanPrefix != "" && strings.Contains(dataStr, "io.gopod.service="+cleanPrefix+"-")) ||
					(cleanPrefix != "" && strings.Contains(dataStr, "io.gopod.service="+cleanPrefix)) ||
					(s.Name != "" && strings.Contains(dataStr, "Label=io.gopod.service="+s.Name)) ||
					(s.ProjectID != "" && s.Name != "" && strings.Contains(dataStr, fmt.Sprintf("io.gopod.project=%s", s.ProjectID)) && strings.Contains(dataStr, s.Name)) {
					isMatch = true
				}
				cName := extractDirective(dataStr, "ContainerName")
				if cName != "" && (seenContainers[cName] || seenContainers[SanitizeQuadletSlug(cName)]) {
					isMatch = true
				}
				pName := extractDirective(dataStr, "Pod")
				if pName == "" {
					pName = extractDirective(dataStr, "PodName")
				}
				if pName != "" && (seenContainers[pName] || seenContainers[SanitizeQuadletSlug(pName)]) {
					isMatch = true
				}
			}

			if isMatch {
				addFile(fullPath)
				addContainer(stem)
				if ext == ".pod" {
					addUnit(fmt.Sprintf("%s-pod.service", stem))
				} else if ext == ".container" {
					addUnit(fmt.Sprintf("%s.service", stem))
				}
				if data, err := os.ReadFile(fullPath); err == nil {
					dataStr := string(data)
					if cName := extractDirective(dataStr, "ContainerName"); cName != "" {
						addContainer(cName)
						addUnit(cName)
					}
					if pName := extractDirective(dataStr, "Pod"); pName != "" {
						cleanP := strings.TrimSuffix(pName, ".pod")
						addContainer(cleanP)
						addUnit(fmt.Sprintf("%s-pod.service", cleanP))
					}
					if pName := extractDirective(dataStr, "PodName"); pName != "" {
						cleanP := strings.TrimSuffix(pName, ".pod")
						addContainer(cleanP)
						addUnit(fmt.Sprintf("%s-pod.service", cleanP))
					}
				}
			}
		}
	}

	// 5. Runtime Container Discovery
	if d.podmanClient != nil {
		if containers, err := d.podmanClient.GetContainers(context.Background()); err == nil {
			for _, c := range containers {
				isMatch := false
				if c.Labels != nil {
					if (s.ID != "" && c.Labels["io.gopod.service"] == s.ID) ||
						(cleanPrefix != "" && strings.HasPrefix(c.Labels["io.gopod.service"], cleanPrefix)) ||
						(s.ID != "" && c.Labels["com.docker.compose.project"] == s.ID) ||
						(s.ID != "" && c.Labels["io.podman.compose.project"] == s.ID) ||
						(cleanPrefix != "" && strings.HasPrefix(c.Labels["com.docker.compose.project"], cleanPrefix)) ||
						(cleanPrefix != "" && strings.HasPrefix(c.Labels["io.podman.compose.project"], cleanPrefix)) ||
						(s.Name != "" && (c.Labels["com.docker.compose.project"] == s.Name || c.Labels["io.podman.compose.project"] == s.Name)) {
						isMatch = true
					}
				}
				if !isMatch {
					for _, name := range c.Names {
						trimmed := strings.TrimPrefix(name, "/")
						if trimmed == s.ID || trimmed == s.Name || strings.HasPrefix(trimmed, cleanPrefix) || (s.ID != "" && strings.HasPrefix(trimmed, s.ID)) || seenContainers[trimmed] {
							isMatch = true
							break
						}
					}
				}

				if isMatch {
					addContainer(c.ID)
					for _, name := range c.Names {
						addContainer(name)
					}
					if c.Labels != nil {
						if unit := c.Labels["io.podman.quadlet.unit"]; unit != "" {
							stem := strings.TrimSuffix(unit, filepath.Ext(unit))
							addUnit(stem)
							addContainer(stem)
							if systemdQuadletDir != "" {
								addFile(filepath.Join(systemdQuadletDir, unit))
							}
						}
					}
				}
			}
		}

		// 6. Runtime Pod Discovery
		if pods, err := d.podmanClient.GetPods(context.Background()); err == nil {
			for _, p := range pods {
				podSlug := SanitizeQuadletSlug(strings.TrimPrefix(strings.TrimSpace(s.PodID), "pod_"))
				expectedPod := ""
				if podSlug != "" {
					expectedPod = fmt.Sprintf("pod_%s", podSlug)
				}
				isMatch := false
				if p.Name == s.ID || p.Name == s.Name || p.Name == cleanName ||
					p.Name == fmt.Sprintf("pod_%s", cleanName) || p.Name == fmt.Sprintf("pod_%s", s.Name) ||
					(expectedPod != "" && p.Name == expectedPod) ||
					(s.PodID != "" && (p.Name == s.PodID || p.Name == fmt.Sprintf("pod_%s", s.PodID))) ||
					strings.HasPrefix(p.Name, cleanPrefix) || (s.ID != "" && strings.HasPrefix(p.Name, s.ID)) ||
					seenContainers[p.Name] {
					isMatch = true
				}
				if isMatch {
					addContainer(p.Name)
					addUnit(fmt.Sprintf("%s-pod.service", strings.TrimSuffix(p.Name, ".pod")))
				}
			}
		}
	}

	return candidateUnits, candidateContainers, filesToDelete
}
