package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopod/internal/podman"
)

// deployQuadlet writes Quadlet systemd units and generates declarative services.
func (d *Deployer) deployQuadlet(ctx context.Context, s Service, depID, trigger string, logLine func(string)) (*DeployResult, error) {
	quadletDir := filepath.Join(d.dataDir, "quadlets")
	_ = os.MkdirAll(quadletDir, 0755)

	cleanName := QuadletUnitName(s)
	content := d.prepareQuadletContent(s, cleanName)

	// Split bundle into individual unit files (.pod and .container)
	files := SplitQuadletUnits(content, cleanName)

	// 1. Validate Image strictly across container units
	for fname, uContent := range files {
		if strings.HasSuffix(fname, ".container") {
			parsed := ParseQuadlet(uContent)
			img := parsed.Image
			if img == "" {
				img = s.Image
			}
			if err := ValidateContainerImage(img); err != nil {
				// Fallback to compose extraction
				if s.ComposeYaml != "" {
					if extImg := ExtractFirstImageFromCompose(s.ComposeYaml); extImg != "" {
						img = extImg
					}
				}
			}
			if err := ValidateContainerImage(img); err != nil {
				logLine(fmt.Sprintf("❌ Invalid container image in '%s': %v", fname, err))
				return nil, fmt.Errorf("Invalid container image identifier: must not be a compose file or path")
			}
			logLine(fmt.Sprintf("🔍 Unit '%s': image '%s' validated.", fname, img))
		}
	}

	// 2. Pre-flight Dry-Run validation
	logLine("🔍 Running pre-flight Quadlet validation (quadlet -dryrun -user)...")
	valid, dryOut, err := ValidateQuadletFiles(files)
	if !valid || err != nil {
		logLine(fmt.Sprintf("❌ Pre-flight Quadlet dry-run rejected: %v\n%s", err, dryOut))
		return nil, fmt.Errorf("Pre-flight Quadlet validation failed: %v", err)
	}
	logLine("✅ Quadlet syntax verified by podman systemd generator.")
	if trimmed := strings.TrimSpace(dryOut); trimmed != "" {
		if len(trimmed) > 2000 {
			trimmed = trimmed[:2000] + "…[truncated]"
		}
		logLine(fmt.Sprintf("🔍 Quadlet dry-run detail: %s", trimmed))
	}

	// 3. Pre-flight Ghost Container & Unit cleanup (prevent conflict exit code 125 and systemd auto-restart races)
	d.cleanPreflightConflicts(ctx, s, logLine)

	// 4. Determine systemd user quadlet search path and purge legacy space-polluted unit files
	systemdQuadletDir := os.Getenv("QUADLET_DIR")
	if systemdQuadletDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdQuadletDir = filepath.Join(homeDir, ".config", "containers", "systemd")
		}
	}
	if systemdQuadletDir != "" {
		_ = os.MkdirAll(systemdQuadletDir, 0755)
		if s.Name != cleanName {
			_ = os.Remove(filepath.Join(systemdQuadletDir, s.Name+".container"))
			_ = os.Remove(filepath.Join(systemdQuadletDir, s.Name+".pod"))
		}
		if entries, err := os.ReadDir(systemdQuadletDir); err == nil {
			for _, entry := range entries {
				eName := entry.Name()
				if strings.Contains(eName, " ") && (strings.HasSuffix(eName, ".container") || strings.HasSuffix(eName, ".pod")) {
					cleanSlug := SanitizeQuadletSlug(strings.TrimSuffix(strings.TrimSuffix(eName, ".container"), ".pod"))
					if cleanSlug == cleanName {
						_ = os.Remove(filepath.Join(systemdQuadletDir, eName))
					}
				}
			}
		}
	}

	// 5. Write unit files to disk
	var primaryServiceName string
	var podMemberServices []string

	for fname, uContent := range files {
		// Local data backup
		backupPath := filepath.Join(quadletDir, fname)
		_ = os.WriteFile(backupPath, []byte(uContent), 0644)
		logLine(fmt.Sprintf("📄 Quadlet artifact staged: %s", backupPath))

		if systemdQuadletDir != "" {
			_ = os.MkdirAll(systemdQuadletDir, 0755)
			userFile := filepath.Join(systemdQuadletDir, fname)
			if err := os.WriteFile(userFile, []byte(uContent), 0644); err != nil {
				logLine(fmt.Sprintf("❌ Failed to write unit '%s': %v", fname, err))
				return nil, err
			}
			logLine(fmt.Sprintf("📄 Written unit to %s", userFile))
		}

		if strings.HasSuffix(fname, ".pod") {
			basePod := strings.TrimSuffix(fname, ".pod")
			primaryServiceName = fmt.Sprintf("%s-pod.service", basePod)
		} else if strings.HasSuffix(fname, ".container") {
			baseMember := strings.TrimSuffix(fname, ".container")
			podMemberServices = append(podMemberServices, fmt.Sprintf("%s.service", baseMember))
		}
	}

	if primaryServiceName == "" && len(podMemberServices) > 0 {
		primaryServiceName = podMemberServices[0]
		podMemberServices = podMemberServices[1:]
	} else if primaryServiceName == "" {
		primaryServiceName = fmt.Sprintf("%s.service", cleanName)
	}
	emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "quadlet.units_written",
		fmt.Sprintf("Quadlet units written for service '%s': %d file(s), primary %s", s.Name, len(files), primaryServiceName),
		map[string]any{"service": s.Name, "files": len(files), "primary": primaryServiceName, "dir": systemdQuadletDir})

	// 6. Pull image first with streaming progress
	mainImage := s.Image
	for _, uContent := range files {
		p := ParseQuadlet(uContent)
		if p.Image != "" {
			mainImage = p.Image
			break
		}
	}
	if mainImage == "" {
		mainImage = "docker.io/library/nginx:alpine"
	}

	// Smart Image Cache Check: avoid redundant remote pulls when image exists locally
	hasImage := false
	if d.podmanClient != nil {
		chkCmd := exec.CommandContext(ctx, "podman", "image", "exists", mainImage)
		d.setupCmdEnv(chkCmd)
		if err := chkCmd.Run(); err == nil {
			hasImage = true
		}
	}

	shouldPull := !hasImage || trigger == "pull" || trigger == "force-pull" || trigger == "update"
	if shouldPull {
		logLine(fmt.Sprintf("📦 Pulling container image '%s'...", mainImage))
		var pullArgs []string
		if s.RegistryID != "" && d.registryProvider != nil {
			if reg, err := d.registryProvider.GetRegistry(ctx, s.RegistryID); err == nil && reg != nil && reg.Username != "" {
				logLine(fmt.Sprintf("🔑 Authenticating with registry '%s' (%s)...", reg.Name, reg.Username))
				pullArgs = append(pullArgs, "--creds", fmt.Sprintf("%s:%s", reg.Username, reg.Token))
			}
		}
		pullArgs = append(pullArgs, "pull", mainImage)
		pullCmd := exec.CommandContext(ctx, "podman", pullArgs...)
		d.setupCmdEnv(pullCmd)
		pStdout, _ := pullCmd.StdoutPipe()
		pStderr, _ := pullCmd.StderrPipe()
		if err := pullCmd.Start(); err == nil {
			d.streamPipes(pStdout, pStderr, logLine)
			_ = pullCmd.Wait()
		}
	} else {
		logLine(fmt.Sprintf("⚡ Container image '%s' is cached locally. Reusing existing image.", mainImage))
	}

	// 7. Reload systemd daemon and activate service (native binary or
	// D-Bus user bus when the engine runs containerized with the host bus).
	systemdSuccess := false
	if d.hasUserManager() {
		logLine("⚙️ Executing: systemctl --user daemon-reload...")
		if out, err := d.systemctlUser(ctx, "daemon-reload"); err != nil {
			logLine(fmt.Sprintf("⚠️ daemon-reload notice: %v (%s)", err, strings.TrimSpace(out)))
		}

		// Reset failed counter so clean start is permitted
		_, _ = d.systemctlUser(ctx, "reset-failed", primaryServiceName)

		logLine(fmt.Sprintf("🚀 Activating systemd service: %s...", primaryServiceName))
		if out, err := d.systemctlUser(ctx, "restart", primaryServiceName); err != nil {
			logLine(fmt.Sprintf("❌ systemctl restart failed: %v (%s)", err, strings.TrimSpace(out)))
			return nil, fmt.Errorf("systemctl start failed: %s", strings.TrimSpace(out))
		}

		// Also start member services if part of a Pod bundle
		for _, memberSvc := range podMemberServices {
			if out, err := d.systemctlUser(ctx, "restart", memberSvc); err != nil {
				logLine(fmt.Sprintf("⚠️ Member service '%s' restart notice: %v (%s)", memberSvc, err, strings.TrimSpace(out)))
			} else {
				logLine(fmt.Sprintf("✅ Member service '%s' restarted under systemd.", memberSvc))
			}
		}

		systemdSuccess = true
		logLine(fmt.Sprintf("✅ Service '%s' activated successfully under systemd user supervisor.", primaryServiceName))
		logLine(fmt.Sprintf("🔧 Supervisor path taken: systemd user manager (primary=%s members=%v).", primaryServiceName, podMemberServices))
		emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "quadlet.systemd_activate",
			fmt.Sprintf("Quadlet units activated via systemd: primary %s members %v", primaryServiceName, podMemberServices), map[string]any{"primary": primaryServiceName, "members": podMemberServices, "dir": systemdQuadletDir})
	}

	deployedContName := cleanName
	if primaryServiceName != "" {
		deployedContName = strings.TrimSuffix(primaryServiceName, ".service")
		deployedContName = strings.TrimSuffix(deployedContName, "-pod")
	}

	if !systemdSuccess {
		// No systemd user manager reachable at all (neither binary nor
		// D-Bus user bus): host-side activation is impossible from here.
		// Keep traffic alive by serving the units as direct containers,
		// and say so explicitly instead of claiming supervision.
		logLine("ℹ️ No systemd user manager reachable from the engine.")
		logLine("🔧 Supervisor path taken: fallback-direct (no systemd user manager; serving units as plain podman containers).")
		logLine(fmt.Sprintf("📄 Quadlet unit written to: %s", systemdQuadletDir))
		emitEngineEventSafe(ctx, "warn", "service", s.ID, s.ProjectID, "engine", "quadlet.fallback_direct",
			fmt.Sprintf("No systemd user manager; quadlet units served directly (%s)", systemdQuadletDir), map[string]any{"dir": systemdQuadletDir})
		if err := d.runQuadletUnitsDirectly(ctx, s, files, logLine); err != nil {
			logLine(fmt.Sprintf("❌ Direct fallback failed: %v", err))
			emitEngineEventSafe(ctx, "error", "service", s.ID, s.ProjectID, "engine", "quadlet.fallback_failed",
				fmt.Sprintf("Direct fallback failed: %v", err), nil)
			return nil, err
		}
		emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "quadlet.fallback_ok",
			fmt.Sprintf("Quadlet units served by direct containers (systemd unsupervised, image %s)", mainImage), map[string]any{"image": mainImage})
		return &DeployResult{
			ContainerName: deployedContName,
			CommitHash:    "",
			CommitMessage: fmt.Sprintf("Quadlet unit compiled; served by direct container, systemd unsupervised (%s)", mainImage),
			Image:         mainImage,
			Version:       "quadlet",
		}, nil
	}

	return &DeployResult{
		ContainerName: deployedContName,
		CommitHash:    "",
		CommitMessage: fmt.Sprintf("Quadlet unit compiled and activated (%s)", mainImage),
		Image:         mainImage,
		Version:       "quadlet",
	}, nil
}

// quadletPublishPorts extracts canonical "[IP:]host:container[/udp]"
// PublishPort bindings from a unit body, preserving non-default bind IPs.
func quadletPublishPorts(content string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		kv := strings.SplitN(trimmed, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(kv[0]), "PublishPort") {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(kv[1]), "\"'")
		if entry := canonicalPortBinding(raw); entry != "" {
			if !seen[entry] {
				seen[entry] = true
				out = append(out, entry)
			}
		}
	}
	return out
}

// runQuadletUnitsDirectly serves Quadlet units as plain podman containers.
// Used only when no systemd user manager is reachable (containerized engine):
// .pod units become real pods (ports on infra), .container members join them,
// standalone units run with their own published ports.
func (d *Deployer) runQuadletUnitsDirectly(ctx context.Context, s Service, files map[string]string, logLine func(string)) error {
	if d.podmanClient == nil {
		return fmt.Errorf("podman client unavailable")
	}
	_ = d.podmanClient.EnsureNetwork(ctx, "gopod-net")

	// Deterministic order: pods first, then members, then standalone.
	names := make([]string, 0, len(files))
	for fname := range files {
		names = append(names, fname)
	}
	sort.Strings(names)

	newPods := map[string]bool{}
	ensurePod := func(podName string, ports []string) error {
		if newPods[podName] {
			return nil
		}
		if _, err := d.podmanClient.InspectPod(ctx, podName); err != nil {
			logLine(fmt.Sprintf("📦 Creating fallback pod '%s' with ports %v...", podName, ports))
			if _, err := d.podmanClient.CreatePod(ctx, podman.CreatePodOptions{
				Name:    podName,
				Ports:   ports,
				Network: "gopod-net",
				Labels: map[string]string{
					"io.gopod.service": s.ID,
					"io.gopod.project": s.ProjectID,
				},
			}); err != nil && !strings.Contains(err.Error(), "already exists") {
				return err
			}
		} else {
			_ = d.podmanClient.StartPod(ctx, podName)
		}
		newPods[podName] = true
		return nil
	}

	// Pass 1: materialize .pod units (infra owns PublishPort).
	for _, fname := range names {
		if !strings.HasSuffix(fname, ".pod") {
			continue
		}
		content := files[fname]
		podName := strings.TrimSuffix(extractDirective(content, "PodName"), ".pod")
		if podName == "" {
			podName = strings.TrimSuffix(filepath.Base(fname), ".pod")
		}
		if err := ensurePod(podName, quadletPublishPorts(content)); err != nil {
			return fmt.Errorf("fallback pod '%s': %w", podName, err)
		}
	}

	// Pass 2: run .container units.
	pulled := map[string]bool{}
	ensureImage := func(image string) {
		if image == "" || pulled[image] {
			return
		}
		pulled[image] = true
		chkCmd := exec.CommandContext(ctx, "podman", "image", "exists", image)
		d.setupCmdEnv(chkCmd)
		if err := chkCmd.Run(); err != nil {
			logLine(fmt.Sprintf("📦 Pulling container image '%s'...", image))
			pullCmd := exec.CommandContext(ctx, "podman", "pull", image)
			d.setupCmdEnv(pullCmd)
			pStdout, _ := pullCmd.StdoutPipe()
			pStderr, _ := pullCmd.StderrPipe()
			if err := pullCmd.Start(); err == nil {
				d.streamPipes(pStdout, pStderr, logLine)
				_ = pullCmd.Wait()
			}
		}
	}

	for _, fname := range names {
		if !strings.HasSuffix(fname, ".container") {
			continue
		}
		content := files[fname]
		parsed := ParseQuadlet(content)
		if parsed.Image == "" {
			return fmt.Errorf("unit '%s' has no Image directive", fname)
		}
		cName := parsed.ContainerName
		if cName == "" {
			cName = strings.TrimSuffix(filepath.Base(fname), ".container")
		}
		ensureImage(parsed.Image)

		targetPod := ""
		var ports []string
		network := parsed.Network
		if network == "" {
			network = "gopod-net"
		}
		if strings.TrimSpace(parsed.Pod) != "" {
			targetPod = strings.TrimSuffix(strings.TrimSpace(parsed.Pod), ".pod")
			if _, err := d.podmanClient.InspectPod(ctx, targetPod); err != nil {
				if err := ensurePod(targetPod, quadletPublishPorts(content)); err != nil {
					return err
				}
			}
			ports = nil
			network = ""
		} else {
			ports = quadletPublishPorts(content)
		}

		var cmdArgs []string
		if strings.TrimSpace(parsed.Command) != "" {
			cmdArgs = strings.Fields(parsed.Command)
		}
		envMap := map[string]string{}
		for k, v := range parsed.Env {
			envMap[k] = v
		}
		restart := ""
		if r := strings.ToLower(strings.TrimSpace(parsed.RestartPolicy)); r == "always" || r == "no" || r == "unless-stopped" || strings.HasPrefix(r, "on-failure") {
			restart = r
		}
		logLine(fmt.Sprintf("🚀 Starting fallback container '%s' (image: %s)...", cName, parsed.Image))
		fallbackOpts := podman.RunContainerOptions{
			Name:    cName,
			Image:   parsed.Image,
			Command: cmdArgs,
			Pod:     targetPod,
			Ports:   ports,
			Env:     envMap,
			Network: network,
			Labels: map[string]string{
				"io.gopod.service": s.ID,
				"io.gopod.project": s.ProjectID,
				"io.gopod.name":    s.Name,
			},
			RestartPolicy: restart,
			Volumes:       parsed.Volumes,
		}
		logLine(fmt.Sprintf("🧾 Equivalent: %s", sanitizedPodmanRunArgs(fallbackOpts)))
		if _, err := d.podmanClient.RunContainer(ctx, fallbackOpts); err != nil {
			logLine(fmt.Sprintf("❌ Fallback container '%s' run failed: %v", cName, err))
			d.enrichRunFailure(ctx, cName, err, logLine)
			emitEngineEventSafe(ctx, "error", "service", s.ID, s.ProjectID, "engine", "quadlet.fallback_container_failed",
				fmt.Sprintf("Fallback container '%s' failed: %v", cName, err), map[string]any{"container": cName, "image": parsed.Image, "pod": targetPod})
			return fmt.Errorf("fallback container '%s': %w", cName, err)
		}
		logLine(fmt.Sprintf("✅ Fallback container '%s' is up.", cName))
		logLine(fmt.Sprintf("✅ Fallback start result: container='%s' image='%s' pod='%s' network='%s' ports=%v.", cName, parsed.Image, targetPod, network, ports))
	}
	return nil
}

// DescribeQuadletSupervision reports the primary systemd unit for a service,
// where its file was written, and whether the host user manager has it active.
func (d *Deployer) DescribeQuadletSupervision(s Service) (unitService, unitFile string, active bool) {
	cleanName := QuadletUnitName(s)
	content := d.prepareQuadletContent(s, cleanName)
	files := SplitQuadletUnits(content, cleanName)
	if len(files) == 0 {
		return "", "", false
	}
	ordered := make([]string, 0, len(files))
	for fname := range files {
		ordered = append(ordered, fname)
	}
	sort.Strings(ordered)
	primaryFile := ""
	for _, fname := range ordered {
		if strings.HasSuffix(fname, ".pod") {
			primaryFile = fname
			break
		}
	}
	if primaryFile == "" {
		for _, fname := range ordered {
			if strings.HasSuffix(fname, ".container") {
				primaryFile = fname
				break
			}
		}
	}
	if primaryFile == "" {
		primaryFile = ordered[0]
	}
	stem := strings.TrimSuffix(primaryFile, filepath.Ext(primaryFile))
	if strings.HasSuffix(primaryFile, ".pod") {
		unitService = fmt.Sprintf("%s-pod.service", stem)
	} else {
		unitService = fmt.Sprintf("%s.service", stem)
	}

	systemdDir := os.Getenv("QUADLET_DIR")
	if systemdDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdDir = filepath.Join(homeDir, ".config", "containers", "systemd")
		}
	}
	if systemdDir != "" {
		unitFile = filepath.Join(systemdDir, primaryFile)
		// Present host-style (~/...) instead of the engine-container home:
		// the systemd dir is bind-mounted to the host user systemd dir.
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			if rel, err := filepath.Rel(homeDir, unitFile); err == nil && !strings.HasPrefix(rel, "..") {
				unitFile = filepath.Join("~", rel)
			}
		}
	}
	return unitService, unitFile, d.isUnitActive(context.Background(), unitService)
}

// GetServiceJournal fetches systemd journalctl entries for the service unit.
func (d *Deployer) GetServiceJournal(ctx context.Context, s Service, lines int) (string, error) {
	if lines <= 0 {
		lines = 100
	}
	unitName := QuadletUnitName(s)
	cands := getQuadletCandidateUnitNames(s, unitName)

	if _, err := exec.LookPath("journalctl"); err != nil {
		return "", fmt.Errorf("journalctl not available on host")
	}

	for _, cand := range cands {
		cmd := exec.CommandContext(ctx, "journalctl", "--user", "-u", cand, "-n", strconv.Itoa(lines), "--no-pager")
		d.setupCmdEnv(cmd)
		out, err := cmd.CombinedOutput()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return string(out), nil
		}
	}

	// Fallback to primary unit name
	cmd := exec.CommandContext(ctx, "journalctl", "--user", "-u", fmt.Sprintf("%s.service", unitName), "-n", strconv.Itoa(lines), "--no-pager")
	d.setupCmdEnv(cmd)
	out, _ := cmd.CombinedOutput()
	return string(out), nil
}
