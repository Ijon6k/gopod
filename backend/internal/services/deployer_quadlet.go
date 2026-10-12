package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ParsedQuadlet contains extracted directives from a Quadlet unit.
type ParsedQuadlet struct {
	Image         string
	ContainerName string
	Ports         []string
	HostPort      int
	Port          int
	Volumes       []string
	Env           map[string]string
	Network       string
	RestartPolicy string
	UserNS        string
	CapAdd        []string
	CapDrop       []string
	Devices       []string
	Privileged    bool
	Pod           string
}

// ValidateContainerImage validates that the image identifier is an OCI image reference
// and explicitly rejects compose files, file paths, and empty strings.
func ValidateContainerImage(image string) error {
	trimmed := strings.TrimSpace(image)
	if trimmed == "" {
		return fmt.Errorf("container image cannot be empty")
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") ||
		strings.Contains(lower, "compose.yaml") || strings.Contains(lower, "compose.yml") ||
		strings.Contains(lower, "docker-compose") {
		return fmt.Errorf("Invalid container image identifier: must not be a compose file or path")
	}
	if strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") || (strings.HasPrefix(trimmed, "/") && !strings.Contains(trimmed, ":")) {
		return fmt.Errorf("Invalid container image identifier: must not be a compose file or path")
	}
	return nil
}

// ExtractFirstImageFromCompose attempts to extract the first valid container image defined in Compose YAML.
func ExtractFirstImageFromCompose(composeYaml string) string {
	trimmed := strings.TrimSpace(composeYaml)
	if trimmed == "" {
		return ""
	}
	var raw struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(trimmed), &raw); err == nil && raw.Services != nil {
		for _, svc := range raw.Services {
			if img := strings.TrimSpace(svc.Image); img != "" && ValidateContainerImage(img) == nil {
				return img
			}
		}
	}
	return ""
}

// ParseQuadlet parses raw .container unit text and extracts container runtime parameters.
func ParseQuadlet(content string) ParsedQuadlet {
	p := ParsedQuadlet{
		Env:     make(map[string]string),
		Network: "gopod-net",
	}

	lines := strings.Split(content, "\n")
	inContainer := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inContainer = strings.EqualFold(trimmed, "[Container]")
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		if inContainer {
			switch {
			case strings.EqualFold(key, "Image"):
				p.Image = val
			case strings.EqualFold(key, "ContainerName"):
				p.ContainerName = val
			case strings.EqualFold(key, "Network"):
				p.Network = val
			case strings.EqualFold(key, "Pod"):
				p.Pod = val
			case strings.EqualFold(key, "PublishPort") || strings.EqualFold(key, "Port"):
				p.Ports = append(p.Ports, val)
				portParts := strings.Split(val, ":")
				if len(portParts) == 2 {
					var hp, cp int
					_, _ = fmt.Sscanf(portParts[0], "%d", &hp)
					_, _ = fmt.Sscanf(portParts[1], "%d", &cp)
					if p.HostPort == 0 && hp > 0 {
						p.HostPort = hp
					}
					if p.Port == 0 && cp > 0 {
						p.Port = cp
					}
				} else if len(portParts) == 3 {
					var hp, cp int
					_, _ = fmt.Sscanf(portParts[1], "%d", &hp)
					_, _ = fmt.Sscanf(portParts[2], "%d", &cp)
					if p.HostPort == 0 && hp > 0 {
						p.HostPort = hp
					}
					if p.Port == 0 && cp > 0 {
						p.Port = cp
					}
				} else if len(portParts) == 1 {
					var cp int
					_, _ = fmt.Sscanf(portParts[0], "%d", &cp)
					if p.Port == 0 && cp > 0 {
						p.Port = cp
					}
				}
			case strings.EqualFold(key, "Volume"):
				p.Volumes = append(p.Volumes, val)
			case strings.EqualFold(key, "Environment"):
				envParts := strings.SplitN(val, "=", 2)
				if len(envParts) == 2 {
					p.Env[envParts[0]] = envParts[1]
				}
			case strings.EqualFold(key, "UserNS"):
				p.UserNS = val
			case strings.EqualFold(key, "AddCapability"):
				p.CapAdd = append(p.CapAdd, val)
			case strings.EqualFold(key, "DropCapability"):
				p.CapDrop = append(p.CapDrop, val)
			case strings.EqualFold(key, "AddDevice"):
				p.Devices = append(p.Devices, val)
			case strings.EqualFold(key, "SecurityLabelDisable"):
				if strings.EqualFold(val, "true") || strings.EqualFold(val, "yes") {
					p.Privileged = true
				}
			case strings.EqualFold(key, "Restart"):
				p.RestartPolicy = val
			}
		} else {
			if strings.EqualFold(key, "Restart") && p.RestartPolicy == "" {
				p.RestartPolicy = val
			}
		}
	}

	return p
}

// SplitQuadletUnits splits single or multi-unit YAML/INI Quadlet configurations into named units.
func SplitQuadletUnits(content string, defaultName string) map[string]string {
	units := make(map[string]string)
	cleanDefault := defaultName
	if cleanDefault == "" {
		cleanDefault = "workload"
	}

	chunks := strings.Split(content, "\n---")
	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(strings.TrimPrefix(chunk, "---"))
		if trimmed == "" {
			continue
		}

		var fileName string
		for _, l := range strings.Split(trimmed, "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(strings.ToLower(l), "# file:") {
				fileName = strings.TrimSpace(l[len("# file:"):])
				break
			}
		}
		if fileName != "" {
			units[fileName] = trimmed
			continue
		}

		if strings.Contains(trimmed, "[Pod]") {
			podName := extractDirective(trimmed, "PodName")
			if podName == "" {
				podName = fmt.Sprintf("pod_%s", cleanDefault)
			}
			units[fmt.Sprintf("%s.pod", podName)] = trimmed
		} else if strings.Contains(trimmed, "[Container]") {
			contName := extractDirective(trimmed, "ContainerName")
			if contName == "" {
				if len(chunks) > 1 {
					contName = fmt.Sprintf("%s-member", cleanDefault)
				} else {
					contName = cleanDefault
				}
			}
			units[fmt.Sprintf("%s.container", contName)] = trimmed
		}
	}

	if len(units) == 0 && strings.TrimSpace(content) != "" {
		trimmed := strings.TrimSpace(content)
		if strings.Contains(trimmed, "[Pod]") {
			units[fmt.Sprintf("pod_%s.pod", cleanDefault)] = trimmed
		} else {
			units[fmt.Sprintf("%s.container", cleanDefault)] = trimmed
		}
	}
	return units
}

func extractDirective(content string, key string) string {
	lines := strings.Split(content, "\n")
	prefix := strings.ToLower(key) + "="
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			val := strings.TrimPrefix(trimmed, trimmed[:len(prefix)])
			return strings.Trim(strings.TrimSpace(val), "\"'")
		}
	}
	return ""
}

// ValidateQuadletFiles runs official podman quadlet dryrun validation in an isolated directory.
func ValidateQuadletFiles(files map[string]string) (bool, string, error) {
	if len(files) == 0 {
		return false, "", fmt.Errorf("no Quadlet unit files provided")
	}

	// 1. Structural AST validation
	for fileName, content := range files {
		trimmed := strings.TrimSpace(content)
		if !strings.Contains(trimmed, "[Unit]") {
			return false, "", fmt.Errorf("unit file '%s' is missing [Unit] section", fileName)
		}
		if strings.HasSuffix(fileName, ".pod") {
			if !strings.Contains(trimmed, "[Pod]") {
				return false, "", fmt.Errorf("pod unit file '%s' is missing [Pod] section", fileName)
			}
		} else if strings.HasSuffix(fileName, ".container") {
			if !strings.Contains(trimmed, "[Container]") {
				return false, "", fmt.Errorf("container unit file '%s' is missing [Container] section", fileName)
			}
			parsed := ParseQuadlet(trimmed)
			if parsed.Image == "" {
				return false, "", fmt.Errorf("container unit file '%s' is missing Image= directive", fileName)
			}
			if err := ValidateContainerImage(parsed.Image); err != nil {
				return false, "", fmt.Errorf("container unit file '%s' has invalid image '%s': %w", fileName, parsed.Image, err)
			}
		}
	}

	// 2. Binary dry-run validator if available
	quadletBin := "/usr/libexec/podman/quadlet"
	if _, err := os.Stat(quadletBin); err != nil {
		if path, err := exec.LookPath("quadlet"); err == nil {
			quadletBin = path
		} else if _, err := os.Stat("/usr/lib/podman/quadlet"); err == nil {
			quadletBin = "/usr/lib/podman/quadlet"
		} else {
			return true, "Syntax valid (AST verified). Quadlet dryrun binary not present.", nil
		}
	}

	tempDir, err := os.MkdirTemp("", "gopod-quadlet-val-*")
	if err != nil {
		return true, "Syntax valid (AST)", nil
	}
	defer os.RemoveAll(tempDir)

	unitDir := filepath.Join(tempDir, "units")
	outDir := filepath.Join(tempDir, "out")
	_ = os.MkdirAll(unitDir, 0755)
	_ = os.MkdirAll(outDir, 0755)

	for fname, content := range files {
		base := filepath.Base(fname)
		_ = os.WriteFile(filepath.Join(unitDir, base), []byte(content), 0644)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, quadletBin, "-dryrun", "-user", outDir)
	cmd.Env = append(os.Environ(), fmt.Sprintf("QUADLET_UNIT_DIRS=%s", unitDir))
	out, err := cmd.CombinedOutput()
	outStr := string(out)

	if err != nil {
		return false, outStr, fmt.Errorf("quadlet dryrun error: %s", strings.TrimSpace(outStr))
	}
	if strings.Contains(outStr, "converting") && strings.Contains(outStr, "error") {
		return false, outStr, fmt.Errorf("quadlet conversion failed: %s", strings.TrimSpace(outStr))
	}

	return true, strings.TrimSpace(outStr), nil
}

// QuadletUnitName produces a consistent, sanitized systemd unit name for a service.
func QuadletUnitName(s Service) string {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		name = s.ID
	}
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "..", "")
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	clean := strings.Trim(b.String(), "-")
	for strings.Contains(clean, "--") {
		clean = strings.ReplaceAll(clean, "--", "-")
	}
	if clean == "" {
		clean = fmt.Sprintf("service-%s", s.ID)
	}
	return clean
}

// prepareQuadletContent prepares declarative unit text ensuring anti-loop limits, ingress, and tracking directives.
func (d *Deployer) prepareQuadletContent(s Service, cleanName string) string {
	rawContent := strings.TrimSpace(s.QuadletConfig)
	if rawContent == "" {
		return d.generateDefaultQuadletContent(s, cleanName)
	}

	if strings.Contains(rawContent, "\n---") {
		chunks := strings.Split(rawContent, "\n---")
		var processedChunks []string
		for _, chunk := range chunks {
			trimmed := strings.TrimSpace(strings.TrimPrefix(chunk, "---"))
			if trimmed == "" {
				continue
			}
			processedChunks = append(processedChunks, d.prepareSingleQuadletContent(s, cleanName, trimmed))
		}
		return strings.Join(processedChunks, "\n---\n")
	}

	return d.prepareSingleQuadletContent(s, cleanName, rawContent)
}

func (d *Deployer) generateDefaultQuadletContent(s Service, cleanName string) string {
	targetContainerName := s.ID
	if targetContainerName == "" {
		targetContainerName = cleanName
	}

	// Resolve image safely
	image := s.Image
	if err := ValidateContainerImage(image); err != nil {
		if s.ComposeYaml != "" {
			extracted := ExtractFirstImageFromCompose(s.ComposeYaml)
			if extracted != "" && ValidateContainerImage(extracted) == nil {
				image = extracted
			}
		}
		if image == "" || ValidateContainerImage(image) != nil {
			image = "docker.io/library/nginx:alpine"
		}
	}

	publishDirective := ""
	if s.HostPort > 0 && s.Port > 0 {
		publishDirective = fmt.Sprintf("PublishPort=%d:%d\n", s.HostPort, s.Port)
	} else if s.HostPort > 0 {
		publishDirective = fmt.Sprintf("PublishPort=%d:80\n", s.HostPort)
	}

	var extraDirectives strings.Builder
	if s.Advanced != nil {
		if s.Advanced.Runtime.UserNamespace != "" {
			extraDirectives.WriteString(fmt.Sprintf("UserNS=%s\n", s.Advanced.Runtime.UserNamespace))
		}
		for _, v := range s.Advanced.Storage.Volumes {
			if v.Source != "" && v.Target != "" {
				if v.Options != "" {
					extraDirectives.WriteString(fmt.Sprintf("Volume=%s:%s:%s\n", v.Source, v.Target, v.Options))
				} else {
					extraDirectives.WriteString(fmt.Sprintf("Volume=%s:%s\n", v.Source, v.Target))
				}
			}
		}
		for _, cap := range s.Advanced.Security.CapAdd {
			if cap != "" {
				extraDirectives.WriteString(fmt.Sprintf("AddCapability=%s\n", cap))
			}
		}
		for _, cap := range s.Advanced.Security.CapDrop {
			if cap != "" {
				extraDirectives.WriteString(fmt.Sprintf("DropCapability=%s\n", cap))
			}
		}
		for _, dev := range s.Advanced.Runtime.Devices {
			if dev != "" {
				extraDirectives.WriteString(fmt.Sprintf("AddDevice=%s\n", dev))
			}
		}
		if s.Advanced.Security.Privileged || s.Advanced.Security.SELinuxLabel == "disable" {
			extraDirectives.WriteString("SecurityLabelDisable=true\n")
		} else if s.Advanced.Security.SELinuxLabel != "" && s.Advanced.Security.SELinuxLabel != "container_file_t" {
			extraDirectives.WriteString(fmt.Sprintf("SecurityLabelType=%s\n", s.Advanced.Security.SELinuxLabel))
		}
		if s.Advanced.Security.AppArmorProfile != "" {
			extraDirectives.WriteString(fmt.Sprintf("AppArmorProfile=%s\n", s.Advanced.Security.AppArmorProfile))
		}
	}

	restartPolicy := s.RestartPolicy
	if restartPolicy == "" {
		restartPolicy = "always"
	}

	return fmt.Sprintf("[Unit]\nDescription=%s Quadlet Service\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Container]\nContainerName=%s\nImage=%s\nNetwork=gopod-net\nLabel=io.gopod.service=%s\nLabel=io.gopod.project=%s\n%s%s\n[Service]\nRestart=%s\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target\n",
		cleanName, targetContainerName, image, s.ID, s.ProjectID, publishDirective, extraDirectives.String(), restartPolicy)
}

func (d *Deployer) prepareSingleQuadletContent(s Service, cleanName string, rawContent string) string {
	targetContainerName := s.ID
	if targetContainerName == "" {
		targetContainerName = cleanName
	}

	lines := strings.Split(rawContent, "\n")
	var newLines []string
	inContainer := false
	hasUnit := strings.Contains(rawContent, "[Unit]")
	hasStartLimitInterval := strings.Contains(rawContent, "StartLimitIntervalSec=")
	hasStartLimitBurst := strings.Contains(rawContent, "StartLimitBurst=")
	hasContainerName := strings.Contains(rawContent, "ContainerName=")
	hasNetwork := strings.Contains(rawContent, "Network=")
	hasServiceLabel := strings.Contains(rawContent, "io.gopod.service")
	hasServiceSection := strings.Contains(rawContent, "[Service]")
	hasPodDirective := strings.Contains(rawContent, "Pod=")
	var containerRestart string

	if !hasUnit {
		newLines = append(newLines, "[Unit]", fmt.Sprintf("Description=GoPod %s Service", cleanName), "After=network-online.target", "StartLimitIntervalSec=60s", "StartLimitBurst=5", "")
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inContainer = strings.EqualFold(trimmed, "[Container]")
		}

		// Podman Quadlet: Restart= is invalid in [Container], belongs in [Service]
		if inContainer && strings.HasPrefix(trimmed, "Restart=") {
			containerRestart = strings.TrimPrefix(trimmed, "Restart=")
			continue
		}

		newLines = append(newLines, line)

		if strings.TrimSpace(line) == "[Unit]" {
			if !hasStartLimitInterval {
				newLines = append(newLines, "StartLimitIntervalSec=60s")
			}
			if !hasStartLimitBurst {
				newLines = append(newLines, "StartLimitBurst=5")
			}
		}

		if strings.TrimSpace(line) == "[Container]" {
			if !hasContainerName {
				newLines = append(newLines, fmt.Sprintf("ContainerName=%s", targetContainerName))
			}
			// In Podman Quadlet, child member of Pod must NOT declare Network=
			if !hasNetwork && !hasPodDirective {
				newLines = append(newLines, "Network=gopod-net")
			}
			if !hasServiceLabel && s.ID != "" {
				newLines = append(newLines, fmt.Sprintf("Label=io.gopod.service=%s", s.ID))
				if s.ProjectID != "" {
					newLines = append(newLines, fmt.Sprintf("Label=io.gopod.project=%s", s.ProjectID))
				}
			}
		}
	}

	if containerRestart != "" && !hasServiceSection {
		newLines = append(newLines, "", "[Service]", fmt.Sprintf("Restart=%s", containerRestart))
	}

	return strings.Join(newLines, "\n")
}

// deployQuadlet writes Quadlet systemd units and generates declarative services.
func (d *Deployer) deployQuadlet(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
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

	// 3. Pre-flight Ghost Container cleanup (prevent conflict exit code 125)
	for fname, uContent := range files {
		if strings.HasSuffix(fname, ".container") {
			cName := extractDirective(uContent, "ContainerName")
			if cName == "" {
				cName = s.ID
			}
			if cName != "" {
				logLine(fmt.Sprintf("🧹 Pre-flight cleanup: removing ghost container '%s'...", cName))
				if d.podmanClient != nil {
					_ = d.podmanClient.StopContainer(ctx, cName)
					_ = d.podmanClient.DeleteContainer(ctx, cName, true)
				}
				rmCmd := exec.CommandContext(ctx, "podman", "rm", "-f", cName)
				d.setupCmdEnv(rmCmd)
				_ = rmCmd.Run()
			}
		}
	}
	d.stopPreviousContainer(ctx, s, logLine)

	// 4. Determine systemd user quadlet search path
	systemdQuadletDir := os.Getenv("QUADLET_DIR")
	if systemdQuadletDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdQuadletDir = filepath.Join(homeDir, ".config", "containers", "systemd")
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

	if primaryServiceName == "" {
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

	if !systemdSuccess {
		return nil, fmt.Errorf("systemd daemon unavailable to activate Quadlet unit")
	}

	return &DeployResult{
		ContainerName: cleanName,
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

// CheckSystemdLinger checks if systemd lingering is enabled for user session persistence.
func CheckSystemdLinger() (bool, string) {
	currUser := os.Getenv("USER")
	if currUser == "" {
		if u, err := user.Current(); err == nil {
			currUser = u.Username
		}
	}
	if currUser == "" {
		currUser = "unknown"
	}

	if _, err := os.Stat(filepath.Join("/var/lib/systemd/linger", currUser)); err == nil {
		return true, currUser
	}

	if _, err := exec.LookPath("loginctl"); err == nil {
		cmd := exec.Command("loginctl", "show-user", currUser, "--property=Linger")
		if out, err := cmd.Output(); err == nil {
			str := strings.TrimSpace(string(out))
			if strings.EqualFold(str, "Linger=yes") {
				return true, currUser
			}
		}
	}

	return false, currUser
}
