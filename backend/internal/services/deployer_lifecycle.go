package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StartService powers on workload containers or units according to service workload type.
func (d *Deployer) StartService(ctx context.Context, s Service) error {
	// 0. Kubernetes Manifest
	if d.isKubeWorkload(s) {
		return d.startKubeService(ctx, s)
	}

	// 1. Quadlet Service (Takes precedence if active runtime is quadlet)
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || (s.Type != "compose" && strings.TrimSpace(s.QuadletConfig) != "" && strings.TrimSpace(s.ComposeYaml) == "") {
		unitName := QuadletUnitName(s)
		cands := getQuadletCandidateUnitNames(s, unitName)
		if d.hasUserManager() {
			started := false
			for _, cand := range cands {
				if out, err := d.systemctlUser(ctx, "start", cand); err == nil {
					started = true
					break
				} else {
					log.Printf("[DEBUG] systemctl start %s: %s (%v)", cand, strings.TrimSpace(out), err)
				}
			}
			if started {
				return nil
			}
		}
		if d.podmanClient != nil {
			if err := d.podmanClient.StartContainer(ctx, s.ID); err == nil {
				return nil
			}
			if err := d.podmanClient.StartContainer(ctx, unitName); err == nil {
				return nil
			}
			parsed := ParseQuadlet(s.QuadletConfig)
			if parsed.ContainerName != "" {
				if err := d.podmanClient.StartContainer(ctx, parsed.ContainerName); err == nil {
					return nil
				}
			}
			if parsed.Pod != "" {
				_ = d.podmanClient.StartPod(ctx, parsed.Pod)
			}
		}
		_, err := d.DeployService(ctx, s, fmt.Sprintf("start-%d", time.Now().Unix()))
		return err
	}

	// 2. Compose Stack
	if d.isComposeWorkload(s) {
		return d.startComposeService(ctx, s)
	}

	// 3. Container / Git / Direct Image / Pod Workload
	if d.podmanClient != nil {
		if s.Type == "pod" || s.RuntimeTarget == "pod" || s.InPod {
			_ = d.podmanClient.StartPod(ctx, s.ID)
			_ = d.podmanClient.StartPod(ctx, s.Name)
			_ = d.podmanClient.StartPod(ctx, fmt.Sprintf("pod_%s", s.ID))
			_ = d.podmanClient.StartPod(ctx, fmt.Sprintf("pod_%s", s.Name))
			_ = d.podmanClient.StartPod(ctx, fmt.Sprintf("pod_%s", SanitizeQuadletSlug(s.Name)))
			_ = d.podmanClient.StartPod(ctx, fmt.Sprintf("%s-%s", s.ProjectID, s.Name))
			if s.PodID != "" {
				_ = d.podmanClient.StartPod(ctx, s.PodID)
				_ = d.podmanClient.StartPod(ctx, fmt.Sprintf("pod_%s", s.PodID))
				podSlug := SanitizeQuadletSlug(strings.TrimPrefix(s.PodID, "pod_"))
				if podSlug != "" {
					_ = d.podmanClient.StartPod(ctx, fmt.Sprintf("pod_%s", podSlug))
				}
			}
		}

		containers, _ := d.podmanClient.GetContainers(ctx)
		found := false
		var startErr error
		for _, c := range containers {
			isMatch := false
			if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID) {
				isMatch = true
			} else if len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if strings.HasPrefix(firstName, s.ID) || firstName == s.Name || strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) {
					isMatch = true
				}
			}
			if isMatch {
				found = true
				if err := d.podmanClient.StartContainer(ctx, c.ID); err != nil {
					startErr = err
				}
			}
		}
		if found {
			return startErr
		}
	}

	// Fallback to fresh deployment if no container exists
	_, err := d.DeployService(ctx, s, fmt.Sprintf("start-%d", time.Now().Unix()))
	return err
}

// StopService gracefully halts containers or units for the service workload.
func (d *Deployer) StopService(ctx context.Context, s Service) error {
	// 0. Kubernetes Manifest
	if d.isKubeWorkload(s) {
		return d.stopKubeService(ctx, s)
	}

	// 1. Quadlet Service (Takes precedence if active runtime is quadlet)
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || (s.Type != "compose" && strings.TrimSpace(s.QuadletConfig) != "" && strings.TrimSpace(s.ComposeYaml) == "") {
		unitName := QuadletUnitName(s)
		parsed := ParseQuadlet(s.QuadletConfig)
		cands := getQuadletCandidateUnitNames(s, unitName)
		if d.hasUserManager() {
			for _, cand := range cands {
				_, _ = d.systemctlUser(ctx, "stop", cand)
			}
		}
		if d.podmanClient != nil {
			if parsed.Pod != "" {
				_ = d.podmanClient.StopPod(ctx, parsed.Pod)
				_ = d.podmanClient.StopPod(ctx, fmt.Sprintf("pod_%s", parsed.Pod))
			}
			_ = d.podmanClient.StopPod(ctx, s.ID)
			_ = d.podmanClient.StopPod(ctx, s.Name)
			_ = d.podmanClient.StopPod(ctx, unitName)
			_ = d.podmanClient.StopPod(ctx, fmt.Sprintf("pod_%s", unitName))

			if parsed.ContainerName != "" {
				_ = d.podmanClient.StopContainer(ctx, parsed.ContainerName)
			}
			_ = d.podmanClient.StopContainer(ctx, s.ID)
			_ = d.podmanClient.StopContainer(ctx, unitName)
			_ = d.podmanClient.StopContainer(ctx, fmt.Sprintf("systemd-%s", unitName))
		}
		return nil
	}

	// 2. Compose Stack
	if d.isComposeWorkload(s) {
		return d.stopComposeService(ctx, s)
	}

	// 3. Container / Git / Direct Image / Pod Workload
	if d.podmanClient != nil {
		if s.Type == "pod" || s.RuntimeTarget == "pod" || s.InPod {
			_ = d.podmanClient.StopPod(ctx, PodNameForService(s))
			_ = d.podmanClient.StopPod(ctx, s.ID)
			_ = d.podmanClient.StopPod(ctx, s.Name)
			_ = d.podmanClient.StopPod(ctx, fmt.Sprintf("pod_%s", s.ID))
			_ = d.podmanClient.StopPod(ctx, fmt.Sprintf("pod_%s", s.Name))
			_ = d.podmanClient.StopPod(ctx, fmt.Sprintf("pod_%s", SanitizeQuadletSlug(s.Name)))
			_ = d.podmanClient.StopPod(ctx, fmt.Sprintf("%s-%s", s.ProjectID, s.Name))
			if s.PodID != "" {
				_ = d.podmanClient.StopPod(ctx, s.PodID)
				_ = d.podmanClient.StopPod(ctx, fmt.Sprintf("pod_%s", s.PodID))
				podSlug := SanitizeQuadletSlug(strings.TrimPrefix(s.PodID, "pod_"))
				if podSlug != "" {
					_ = d.podmanClient.StopPod(ctx, fmt.Sprintf("pod_%s", podSlug))
				}
			}
			for _, leg := range legacyPodCandidates(s) {
				_ = d.podmanClient.StopPod(ctx, leg)
			}
		}

		containers, _ := d.podmanClient.GetContainers(ctx)
		cleanPrefix := fmt.Sprintf("%s-%s", s.ProjectID, s.Name)
		candUnits := getQuadletCandidateUnitNames(s, QuadletUnitName(s))

		for _, c := range containers {
			isMatch := false
			if c.Labels != nil {
				if c.Labels["io.gopod.service"] == s.ID ||
					c.Labels["com.docker.compose.project"] == s.ID ||
					c.Labels["io.podman.compose.project"] == s.ID ||
					c.Labels["com.docker.compose.project"] == s.Name {
					isMatch = true
				}
				for _, cand := range candUnits {
					baseUnit := strings.TrimSuffix(cand, ".service")
					if c.Labels["io.podman.quadlet.unit"] == fmt.Sprintf("%s.container", baseUnit) ||
						c.Labels["io.podman.quadlet.unit"] == fmt.Sprintf("%s.pod", baseUnit) {
						isMatch = true
					}
				}
			}
			if !isMatch && len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if strings.HasPrefix(firstName, s.ID) ||
					firstName == s.Name ||
					firstName == QuadletUnitName(s) ||
					strings.HasPrefix(firstName, cleanPrefix) ||
					firstName == fmt.Sprintf("systemd-%s", QuadletUnitName(s)) {
					isMatch = true
				}
				parsed := ParseQuadlet(s.QuadletConfig)
				if parsed.ContainerName != "" && firstName == parsed.ContainerName {
					isMatch = true
				}
			}
			if isMatch {
				_ = d.podmanClient.StopContainer(ctx, c.ID)
			}
		}
	}

	return nil
}

// RestartService restarts running containers or units for the service workload.
func (d *Deployer) RestartService(ctx context.Context, s Service) error {
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || (s.Type != "compose" && strings.TrimSpace(s.QuadletConfig) != "") {
		unitName := QuadletUnitName(s)
		cands := getQuadletCandidateUnitNames(s, unitName)
		if d.hasUserManager() {
			for _, cand := range cands {
				if _, err := d.systemctlUser(ctx, "restart", cand); err == nil {
					return nil
				}
			}
		}
	}
	_ = d.StopService(ctx, s)
	return d.StartService(ctx, s)
}

// cleanPreflightConflicts stops lingering systemd units and deletes ghost containers before a deployment
// to guarantee zero container name collisions and prevent systemd auto-restart race conditions.
func (d *Deployer) cleanPreflightConflicts(ctx context.Context, s Service, logLine func(string)) {
	candUnits, candContainers, _ := d.findServiceUnitFilesAndCandidates(s)

	if d.hasUserManager() {
		for _, u := range candUnits {
			_, _ = d.systemctlUser(ctx, "stop", u)
			_, _ = d.systemctlUser(ctx, "kill", "--kill-who=all", "--signal=SIGKILL", u)
			_, _ = d.systemctlUser(ctx, "reset-failed", u)
		}
	}

	if d.podmanClient != nil {
		for _, name := range candContainers {
			_ = d.podmanClient.StopContainer(ctx, name)
			_ = d.podmanClient.DeleteContainer(ctx, name, true)
		}
	}
	for _, name := range candContainers {
		cmdRm := exec.CommandContext(ctx, "podman", "rm", "-f", name)
		d.setupCmdEnv(cmdRm)
		_ = cmdRm.Run()
	}
	if logLine != nil && len(candContainers) > 0 {
		logLine(fmt.Sprintf("🧹 Pre-flight cleanup: reconciled %d candidate targets (%s)", len(candContainers), strings.Join(candContainers, ", ")))
	}
}

// TeardownService stops and completely removes containers, pods, quadlets, compose stacks, and kube workloads.
func (d *Deployer) TeardownService(ctx context.Context, s Service, deleteVolumes bool) error {
	// Independent detached context with generous timeout to guarantee clean SIGKILL and socket shutdown
	teardownCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Phase 0: Discover ALL candidate units, containers, and unit files on disk
	candUnits, candContainers, filesToDelete := d.findServiceUnitFilesAndCandidates(s)

	// Phase 1: Force Stop and Reset all candidate Systemd Units WHILE unit files still exist on disk!
	if d.hasUserManager() {
		for _, cand := range candUnits {
			_, _ = d.systemctlUser(teardownCtx, "stop", cand)
			_, _ = d.systemctlUser(teardownCtx, "kill", "--kill-who=all", "--signal=SIGKILL", cand)
			_, _ = d.systemctlUser(teardownCtx, "reset-failed", cand)
		}
	}

	// Phase 2: Purge ALL discovered unit files from disk
	for _, fpath := range filesToDelete {
		_ = os.Remove(fpath)
	}

	systemdQuadletDir := os.Getenv("QUADLET_DIR")
	if systemdQuadletDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdQuadletDir = filepath.Join(homeDir, ".config", "containers", "systemd")
		}
	}
	quadletDir := filepath.Join(d.dataDir, "quadlets")
	for _, base := range candContainers {
		if systemdQuadletDir != "" {
			_ = os.Remove(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.container", base)))
			_ = os.Remove(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.pod", base)))
			_ = os.Remove(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.volume", base)))
			_ = os.Remove(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.network", base)))
		}
		_ = os.Remove(filepath.Join(quadletDir, fmt.Sprintf("%s.container", base)))
		_ = os.Remove(filepath.Join(quadletDir, fmt.Sprintf("%s.pod", base)))
		_ = os.Remove(filepath.Join(quadletDir, fmt.Sprintf("%s.volume", base)))
		_ = os.Remove(filepath.Join(quadletDir, fmt.Sprintf("%s.network", base)))
	}

	// Phase 3: Systemd Daemon-Reload so podman-system-generator purges generated services
	if d.hasUserManager() {
		_, _ = d.systemctlUser(teardownCtx, "daemon-reload")

		for _, cand := range candUnits {
			_, _ = d.systemctlUser(teardownCtx, "reset-failed", cand)
		}
		_, _ = d.systemctlUser(teardownCtx, "reset-failed")
	}

	// Phase 4: Force Remove Containers & Pods (Systemd is now dead, so Restart=always cannot respawn)
	if d.podmanClient != nil {
		for _, cName := range candContainers {
			_ = d.podmanClient.StopContainer(teardownCtx, cName)
			_ = d.podmanClient.DeleteContainer(teardownCtx, cName, true)
			cmdRm := exec.CommandContext(teardownCtx, "podman", "rm", "-f", cName)
			d.setupCmdEnv(cmdRm)
			_ = cmdRm.Run()
		}

		if containers, err := d.podmanClient.GetContainers(teardownCtx); err == nil {
			cleanPrefix := fmt.Sprintf("%s-%s", s.ProjectID, s.Name)
			for _, c := range containers {
				isMatch := false
				if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID || c.Labels["io.podman.compose.project"] == s.ID) {
					isMatch = true
				} else {
					for _, n := range c.Names {
						trimmed := strings.TrimPrefix(n, "/")
						if trimmed == s.ID || trimmed == s.Name || strings.HasPrefix(trimmed, cleanPrefix) || strings.HasPrefix(trimmed, s.ID) {
							isMatch = true
							break
						}
						for _, candC := range candContainers {
							if trimmed == candC {
								isMatch = true
								break
							}
						}
					}
				}
				if isMatch {
					_ = d.podmanClient.StopContainer(teardownCtx, c.ID)
					_ = d.podmanClient.DeleteContainer(teardownCtx, c.ID, true)
					cmdRm := exec.CommandContext(teardownCtx, "podman", "rm", "-f", c.ID)
					d.setupCmdEnv(cmdRm)
					_ = cmdRm.Run()
				}
			}
		}

		// Force-delete pods
		for _, podCand := range candContainers {
			_ = d.podmanClient.DeletePod(teardownCtx, podCand, true)
			_ = d.podmanClient.DeletePod(teardownCtx, "pod_"+podCand, true)
			cmdPodRm := exec.CommandContext(teardownCtx, "podman", "pod", "rm", "-f", podCand)
			d.setupCmdEnv(cmdPodRm)
			_ = cmdPodRm.Run()
		}
		_ = d.podmanClient.DeletePod(teardownCtx, fmt.Sprintf("pod_%s", s.Name), true)
		_ = d.podmanClient.DeletePod(teardownCtx, fmt.Sprintf("pod_%s", SanitizeQuadletSlug(s.Name)), true)
		if s.PodID != "" {
			_ = d.podmanClient.DeletePod(teardownCtx, s.PodID, true)
			_ = d.podmanClient.DeletePod(teardownCtx, fmt.Sprintf("pod_%s", s.PodID), true)
		}
	}

	// Phase 5: Kubernetes Manifest Teardown
	if d.isKubeWorkload(s) {
		d.teardownKubeService(teardownCtx, s, deleteVolumes)
	}

	// Phase 6: Compose Workload Teardown
	if d.isComposeWorkload(s) {
		d.teardownComposeService(teardownCtx, s, deleteVolumes)
	}

	// Phase 7: Persistent Volume Cleanup (Only if explicitly requested)
	if deleteVolumes && d.podmanClient != nil {
		vols, _ := d.podmanClient.GetVolumes(teardownCtx)
		cleanPrefix := fmt.Sprintf("%s-%s", s.ProjectID, s.Name)
		for _, v := range vols {
			if strings.HasPrefix(v.Name, s.ID) || strings.HasPrefix(v.Name, cleanPrefix) || strings.HasPrefix(v.Name, s.Name) {
				_ = d.podmanClient.DeleteVolume(teardownCtx, v.Name, true)
			}
			for _, cand := range candContainers {
				if v.Name == cand || strings.HasPrefix(v.Name, cand+"_") {
					_ = d.podmanClient.DeleteVolume(teardownCtx, v.Name, true)
				}
			}
		}

		if strings.TrimSpace(s.QuadletConfig) != "" {
			parsed := ParseQuadlet(s.QuadletConfig)
			for _, qv := range parsed.Volumes {
				volName := strings.Split(qv, ":")[0]
				if volName != "" && !strings.HasPrefix(volName, "/") && !strings.HasPrefix(volName, ".") {
					_ = d.podmanClient.DeleteVolume(teardownCtx, volName, true)
				}
			}
		}
	}

	emitEngineEventSafe(teardownCtx, "info", "service", s.ID, s.ProjectID, "engine", "service.teardown_summary",
		fmt.Sprintf("Teardown summary for service '%s': %d unit(s), %d container candidate(s), %d file(s) removed (deleteVolumes=%v)",
			s.Name, len(candUnits), len(candContainers), len(filesToDelete), deleteVolumes),
		map[string]any{"service": s.Name, "units": len(candUnits), "containers": len(candContainers), "files": len(filesToDelete), "deleteVolumes": deleteVolumes})

	return nil
}
