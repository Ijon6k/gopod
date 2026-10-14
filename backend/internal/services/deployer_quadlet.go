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
		_ = os.WriteFile(filepath.Join(quadletDir, fname), []byte(uContent), 0644)

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

	// 7. Reload systemd daemon and activate service
	systemdSuccess := false
	if _, err := exec.LookPath("systemctl"); err == nil {
		logLine("⚙️ Executing: systemctl --user daemon-reload...")
		reloadCmd := exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload")
		d.setupCmdEnv(reloadCmd)
		if out, err := reloadCmd.CombinedOutput(); err != nil {
			logLine(fmt.Sprintf("⚠️ daemon-reload notice: %v (%s)", err, strings.TrimSpace(string(out))))
		}

		// Reset failed counter so clean start is permitted
		resetCmd := exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", primaryServiceName)
		d.setupCmdEnv(resetCmd)
		_ = resetCmd.Run()

		logLine(fmt.Sprintf("🚀 Activating systemd service: %s...", primaryServiceName))
		startCmd := exec.CommandContext(ctx, "systemctl", "--user", "restart", primaryServiceName)
		d.setupCmdEnv(startCmd)
		if out, err := startCmd.CombinedOutput(); err != nil {
			logLine(fmt.Sprintf("❌ systemctl restart failed: %v (%s)", err, strings.TrimSpace(string(out))))
			return nil, fmt.Errorf("systemctl start failed: %s", strings.TrimSpace(string(out)))
		}

		// Also start member services if part of a Pod bundle
		for _, memberSvc := range podMemberServices {
			mStart := exec.CommandContext(ctx, "systemctl", "--user", "restart", memberSvc)
			d.setupCmdEnv(mStart)
			_ = mStart.Run()
		}

		systemdSuccess = true
		logLine(fmt.Sprintf("✅ Service '%s' activated successfully under systemd user supervisor.", primaryServiceName))
	}

	deployedContName := cleanName
	if primaryServiceName != "" {
		deployedContName = strings.TrimSuffix(primaryServiceName, ".service")
		deployedContName = strings.TrimSuffix(deployedContName, "-pod")
	}

	if !systemdSuccess {
		logLine("ℹ️ Containerized environment detected (systemctl not in container PATH).")
		logLine(fmt.Sprintf("📄 Quadlet unit written to: %s", systemdQuadletDir))
		logLine(fmt.Sprintf("💡 On host Fedora/Linux, supervise with: systemctl --user daemon-reload && systemctl --user start %s", primaryServiceName))
		systemdSuccess = true
		logLine(fmt.Sprintf("✅ Quadlet unit '%s' compiled and ready for host systemd supervision.", primaryServiceName))
	}

	return &DeployResult{
		ContainerName: deployedContName,
		CommitHash:    "",
		CommitMessage: fmt.Sprintf("Quadlet unit compiled and activated (%s)", mainImage),
		Image:         mainImage,
		Version:       "quadlet",
	}, nil
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
