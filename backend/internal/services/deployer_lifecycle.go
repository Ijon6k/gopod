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
			// Label-only pod resolution: only pods gopod owns — plus the
			// canonical pod name, which is globally unique — are ever started.
			pods, _ := d.podmanClient.GetPods(ctx)
			for _, p := range pods {
				if !podOwnedBy(p, s) && p.Name != PodNameForService(s) {
					continue
				}
				_ = d.podmanClient.StartPod(ctx, p.Name)
			}
			if s.PodID != "" {
				_ = d.podmanClient.StartPod(ctx, PodNameFor(s.PodID))
			}
			for _, leg := range legacyPodCandidates(s) {
				for _, p := range pods {
					if p.Name == leg && podLegacyCandidateOwned(p, s) {
						_ = d.podmanClient.StartPod(ctx, p.Name)
						break
					}
				}
			}
		}

		containers, _ := d.podmanClient.GetContainers(ctx)
		found := false
		var startErr error
		for _, c := range containers {
			if !containerOwnedBy(c, s) {
				continue
			}
			found = true
			if err := d.podmanClient.StartContainer(ctx, c.ID); err != nil {
				startErr = err
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
			// Label-only pod resolution: never stop another project's
			// pod because of a name collision.
			pods, _ := d.podmanClient.GetPods(ctx)
			for _, p := range pods {
				if !podOwnedBy(p, s) && p.Name != PodNameForService(s) {
					continue
				}
				_ = d.podmanClient.StopPod(ctx, p.Name)
			}
			if s.PodID != "" {
				_ = d.podmanClient.StopPod(ctx, PodNameFor(s.PodID))
			}
			for _, leg := range legacyPodCandidates(s) {
				for _, p := range pods {
					if p.Name == leg && podLegacyCandidateOwned(p, s) {
						_ = d.podmanClient.StopPod(ctx, p.Name)
						break
					}
				}
			}
		}

		// Label-only container matching: unlabeled containers belong to
		// other tools/users and are never touched.
		containers, _ := d.podmanClient.GetContainers(ctx)
		for _, c := range containers {
			if containerOwnedBy(c, s) {
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
		// Ownership guard: candidate names are unit-identity derived, so
		// a foreign container may exist with the same name. Only containers
		// that exist AND carry the gopod ownership labels are removed.
		if ownedByName, err := d.ownedContainersByName(ctx, s); err == nil {
			for _, name := range candContainers {
				c, exists := ownedByName[name]
				if !exists || !containerOwnedBy(c, s) {
					continue
				}
				_ = d.podmanClient.StopContainer(ctx, name)
				if delErr := d.podmanClient.DeleteContainer(ctx, name, true); delErr != nil {
					cmdRm := exec.CommandContext(ctx, "podman", "rm", "-f", name)
					d.setupCmdEnv(cmdRm)
					_ = cmdRm.Run()
				}
			}
		}
	} else {
		// CLI-only path (no REST client): verify ownership through the label
		// filter before any rm, then delete only the intersection.
		labelArgs := []string{"ps", "-a", "--filter", "label=io.gopod.service=" + s.ID, "--format", "{{.Names}}"}
		out, err := exec.CommandContext(ctx, "podman", labelArgs...).CombinedOutput()
		if err != nil {
			out = nil
		}
		owned := map[string]bool{}
		for _, line := range strings.Split(string(out), "\n") {
			n := strings.TrimSpace(line)
			if n != "" {
				owned[strings.TrimPrefix(n, "/")] = true
			}
		}
		for _, name := range candContainers {
			if !owned[name] {
				continue
			}
			cmdRm := exec.CommandContext(ctx, "podman", "rm", "-f", name)
			d.setupCmdEnv(cmdRm)
			_ = cmdRm.Run()
		}
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

	// Teardown keeps going on individual failures, but a refused removal must
	// leave a trail: a silent `_ =` makes "service won't die" cases
	// undiagnosable.
	removeFile := func(path string) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("[teardown] failed to remove %s: %v", path, err)
		}
	}

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
		removeFile(fpath)
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
			removeFile(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.container", base)))
			removeFile(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.pod", base)))
			removeFile(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.volume", base)))
			removeFile(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.network", base)))
		}
		removeFile(filepath.Join(quadletDir, fmt.Sprintf("%s.container", base)))
		removeFile(filepath.Join(quadletDir, fmt.Sprintf("%s.pod", base)))
		removeFile(filepath.Join(quadletDir, fmt.Sprintf("%s.volume", base)))
		removeFile(filepath.Join(quadletDir, fmt.Sprintf("%s.network", base)))
	}

	// Phase 3: Systemd Daemon-Reload so podman-system-generator purges generated services
	if d.hasUserManager() {
		_, _ = d.systemctlUser(teardownCtx, "daemon-reload")

		for _, cand := range candUnits {
			_, _ = d.systemctlUser(teardownCtx, "reset-failed", cand)
		}
		_, _ = d.systemctlUser(teardownCtx, "reset-failed")
	}

	// Phase 4: Force Remove Containers & Pods (systemd units are stopped, so Restart=always cannot respawn)
	if d.podmanClient != nil {
		// Ownership guard: candidate container names are unit-identity
		// derived (systemd unit names collide across projects), so a NAME match
		// alone must never delete a foreign container. Resolve the name to an
		// actual container and require the gopod ownership labels first.
		ownedByName, err := d.ownedContainersByName(teardownCtx, s)
		if err == nil {
			for _, cName := range candContainers {
				c, ok := ownedByName[cName]
				if !ok || !containerOwnedBy(c, s) {
					continue
				}
				_ = d.podmanClient.StopContainer(teardownCtx, cName)
				// CLI fork only as fallback when the REST delete fails.
				if delErr := d.podmanClient.DeleteContainer(teardownCtx, cName, true); delErr != nil {
					cmdRm := exec.CommandContext(teardownCtx, "podman", "rm", "-f", cName)
					d.setupCmdEnv(cmdRm)
					if rmErr := cmdRm.Run(); rmErr != nil {
						log.Printf("[teardown] failed to remove container %s (REST: %v, CLI: %v)", cName, delErr, rmErr)
					}
				}
			}
		}

		if containers, err := d.podmanClient.GetContainers(teardownCtx); err == nil {
			// Label-only matching: unlabeled containers are not ours;
			// owned-name resolution above already handled exact candidates.
			for _, c := range containers {
				if !containerOwnedBy(c, s) {
					continue
				}
				_ = d.podmanClient.StopContainer(teardownCtx, c.ID)
				if delErr := d.podmanClient.DeleteContainer(teardownCtx, c.ID, true); delErr != nil {
					cmdRm := exec.CommandContext(teardownCtx, "podman", "rm", "-f", c.ID)
					d.setupCmdEnv(cmdRm)
					if rmErr := cmdRm.Run(); rmErr != nil {
						log.Printf("[teardown] failed to remove container %s (REST: %v, CLI: %v)", c.ID, delErr, rmErr)
					}
				}
			}
		}

		// Force-delete pods, strictly ownership-checked: only pods
		// labeled io.gopod.service==s.ID or named exactly the canonical pod
		// are removed. Cross-project pods are never touched.
		if pods, err := d.podmanClient.GetPods(teardownCtx); err == nil {
			canonicalPod := PodNameForService(s)
			for _, p := range pods {
				if !podOwnedBy(p, s) && p.Name != canonicalPod {
					continue
				}
				if delErr := d.podmanClient.DeletePod(teardownCtx, p.Name, true); delErr != nil {
					cmdPodRm := exec.CommandContext(teardownCtx, "podman", "pod", "rm", "-f", p.Name)
					d.setupCmdEnv(cmdPodRm)
					if rmErr := cmdPodRm.Run(); rmErr != nil {
						log.Printf("[teardown] failed to remove pod %s (REST: %v, CLI: %v)", p.Name, delErr, rmErr)
					}
				}
			}
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
		for _, v := range vols {
			// Label-only ownership: bare name prefixes could delete a
			// sibling service's volumes (web vs web2). Unlabeled volumes are
			// only removed when they carry an exact unit-file candidate name.
			owned := volumeOwnedBy(v, s)
			if !owned {
				for _, cand := range candContainers {
					if v.Name == cand || strings.HasPrefix(v.Name, cand+"_") {
						owned = true
						break
					}
				}
			}
			if owned {
				if err := d.podmanClient.DeleteVolume(teardownCtx, v.Name, true); err != nil {
					log.Printf("[teardown] failed to remove volume %s: %v", v.Name, err)
				}
			}
		}

		if strings.TrimSpace(s.QuadletConfig) != "" {
			parsed := ParseQuadlet(s.QuadletConfig)
			for _, qv := range parsed.Volumes {
				volName := strings.Split(qv, ":")[0]
				if volName != "" && !strings.HasPrefix(volName, "/") && !strings.HasPrefix(volName, ".") {
					if err := d.podmanClient.DeleteVolume(teardownCtx, volName, true); err != nil {
						log.Printf("[teardown] failed to remove quadlet volume %s: %v", volName, err)
					}
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
