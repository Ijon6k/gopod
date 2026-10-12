package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopod/internal/podman"
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

// prepareQuadletContent prepares declarative unit text ensuring Caddy ingress and tracking directives.
func (d *Deployer) prepareQuadletContent(s Service, cleanName string) string {
	rawContent := strings.TrimSpace(s.QuadletConfig)
	targetContainerName := s.ID
	if targetContainerName == "" {
		targetContainerName = cleanName
	}

	if rawContent == "" {
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

		image := s.Image
		if image == "" {
			image = "docker.io/library/nginx:alpine"
		}

		return fmt.Sprintf("[Unit]\nDescription=%s Quadlet Service\nAfter=network-online.target\n\n[Container]\nContainerName=%s\nImage=%s\nNetwork=gopod-net\nLabel=io.gopod.service=%s\nLabel=io.gopod.project=%s\n%s%s\n[Service]\nRestart=%s\n\n[Install]\nWantedBy=default.target\n",
			cleanName, targetContainerName, image, s.ID, s.ProjectID, publishDirective, extraDirectives.String(), restartPolicy)
	}

	// For custom unit configuration: ensure required directives exist under [Container]
	if strings.Contains(rawContent, "[Container]") {
		lines := strings.Split(rawContent, "\n")
		var newLines []string
		inContainer := false
		hasContainerName := strings.Contains(rawContent, "ContainerName=")
		hasNetwork := strings.Contains(rawContent, "Network=")
		hasServiceLabel := strings.Contains(rawContent, "io.gopod.service")
		hasServiceSection := strings.Contains(rawContent, "[Service]")
		var containerRestart string

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
			if strings.TrimSpace(line) == "[Container]" {
				if !hasContainerName {
					newLines = append(newLines, fmt.Sprintf("ContainerName=%s", targetContainerName))
				}
				if !hasNetwork {
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

	return rawContent
}

// deployQuadlet writes Quadlet systemd unit and generates declarative service.
func (d *Deployer) deployQuadlet(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
	quadletDir := filepath.Join(d.dataDir, "quadlets")
	_ = os.MkdirAll(quadletDir, 0755)

	cleanName := QuadletUnitName(s)
	content := d.prepareQuadletContent(s, cleanName)
	parsed := ParseQuadlet(content)

	imageToRun := parsed.Image
	if imageToRun == "" {
		imageToRun = s.Image
	}
	if imageToRun == "" {
		imageToRun = "docker.io/library/nginx:alpine"
	}
	s.Image = imageToRun
	if parsed.HostPort > 0 {
		s.HostPort = parsed.HostPort
	}
	if parsed.Port > 0 {
		s.Port = parsed.Port
	}

	targetContainerName := parsed.ContainerName
	if targetContainerName == "" {
		targetContainerName = s.ID
	}
	if targetContainerName == "" {
		targetContainerName = cleanName
	}

	// 1. Pull image first with streaming progress
	logLine(fmt.Sprintf("📦 Pulling container image '%s'...", imageToRun))
	var pullArgs []string
	if s.RegistryID != "" && d.registryProvider != nil {
		if reg, err := d.registryProvider.GetRegistry(ctx, s.RegistryID); err == nil && reg != nil && reg.Username != "" {
			logLine(fmt.Sprintf("🔑 Authenticating with registry '%s' (%s)...", reg.Name, reg.Username))
			pullArgs = append(pullArgs, "--creds", fmt.Sprintf("%s:%s", reg.Username, reg.Token))
		}
	}
	pullArgs = append(pullArgs, "pull", imageToRun)
	pullCmd := exec.CommandContext(ctx, "podman", pullArgs...)
	d.setupCmdEnv(pullCmd)
	pStdout, _ := pullCmd.StdoutPipe()
	pStderr, _ := pullCmd.StderrPipe()
	if err := pullCmd.Start(); err == nil {
		d.streamPipes(pStdout, pStderr, logLine)
		if err := pullCmd.Wait(); err != nil {
			logLine(fmt.Sprintf("❌ Failed to pull image '%s': %v", imageToRun, err))
			return nil, fmt.Errorf("failed to pull image %s: %w", imageToRun, err)
		}
	} else {
		logLine(fmt.Sprintf("⚠️ Unable to run podman pull: %v", err))
	}
	logLine(fmt.Sprintf("✅ Container image '%s' ready in local storage.", imageToRun))

	// 2. Stop and clean previous containers to avoid name/port conflicts
	d.stopPreviousContainer(ctx, s, logLine)
	if d.podmanClient != nil && targetContainerName != "" && targetContainerName != s.ID {
		_ = d.podmanClient.StopContainer(ctx, targetContainerName)
		_ = d.podmanClient.DeleteContainer(ctx, targetContainerName, true)
	}

	// 3. Write Quadlet unit to GOPOD data directory
	unitFile := filepath.Join(quadletDir, fmt.Sprintf("%s.container", cleanName))
	if err := os.WriteFile(unitFile, []byte(content), 0644); err != nil {
		logLine(fmt.Sprintf("❌ Failed to write quadlet file: %v", err))
		return nil, err
	}
	logLine(fmt.Sprintf("📄 Quadlet unit written to %s", unitFile))

	// 4. Install into systemd user quadlet search path
	systemdQuadletDir := os.Getenv("QUADLET_DIR")
	if systemdQuadletDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdQuadletDir = filepath.Join(homeDir, ".config", "containers", "systemd")
		}
	}
	if systemdQuadletDir != "" {
		_ = os.MkdirAll(systemdQuadletDir, 0755)
		userUnitFile := filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.container", cleanName))
		if err := os.WriteFile(userUnitFile, []byte(content), 0644); err == nil {
			logLine(fmt.Sprintf("📄 Installed Quadlet unit into systemd path: %s", userUnitFile))
		}
	}

	// 5. Attempt systemd reload if systemctl is available
	systemdSuccess := false
	if _, err := exec.LookPath("systemctl"); err == nil {
		logLine("⚙️ Executing: systemctl --user daemon-reload...")
		reloadCmd := exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload")
		d.setupCmdEnv(reloadCmd)
		if out, err := reloadCmd.CombinedOutput(); err != nil {
			logLine(fmt.Sprintf("⚠️ daemon-reload notice: %v (%s)", err, strings.TrimSpace(string(out))))
		}
		logLine(fmt.Sprintf("🚀 Starting systemd service: %s.service...", cleanName))
		restartCmd := exec.CommandContext(ctx, "systemctl", "--user", "restart", fmt.Sprintf("%s.service", cleanName))
		d.setupCmdEnv(restartCmd)
		if out, err := restartCmd.CombinedOutput(); err != nil {
			logLine(fmt.Sprintf("⚠️ systemctl restart notice: %v (%s). Falling back to direct Podman engine execution...", err, strings.TrimSpace(string(out))))
		} else {
			systemdSuccess = true
			portInfo := "none"
			if len(parsed.Ports) > 0 {
				portInfo = strings.Join(parsed.Ports, ", ")
			}
			logLine(fmt.Sprintf("✅ Reloaded and activated systemd user daemon for %s.service (network: %s, ports: %s)", cleanName, parsed.Network, portInfo))
		}
	}

	if !systemdSuccess {
		if _, err := exec.LookPath("systemctl"); err != nil {
			logLine("⚠️ Host systemctl not found (containerized runtime). Starting container directly via Podman...")
		}
		if d.podmanClient != nil {
			_ = d.podmanClient.EnsureNetwork(ctx, parsed.Network)

			envMap := make(map[string]string)
			for _, env := range s.EnvVars {
				if env.Key != "" {
					envMap[env.Key] = env.Value
				}
			}
			for k, v := range parsed.Env {
				envMap[k] = v
			}

			labels := map[string]string{
				"io.gopod.project": s.ProjectID,
				"io.gopod.service": s.ID,
				"io.gopod.name":    s.Name,
			}

			ports := parsed.Ports
			if len(ports) == 0 {
				if s.HostPort > 0 && s.Port > 0 {
					ports = append(ports, fmt.Sprintf("%d:%d", s.HostPort, s.Port))
				} else if s.HostPort > 0 {
					ports = append(ports, fmt.Sprintf("%d:%d", s.HostPort, s.HostPort))
				}
			}

			vols := parsed.Volumes
			if s.Advanced != nil {
				for _, v := range s.Advanced.Storage.Volumes {
					if v.Source != "" && v.Target != "" {
						if v.Options != "" {
							vols = append(vols, fmt.Sprintf("%s:%s:%s", v.Source, v.Target, v.Options))
						} else {
							vols = append(vols, fmt.Sprintf("%s:%s", v.Source, v.Target))
						}
					}
				}
			}

			secOpts := []string{}
			if parsed.Privileged {
				secOpts = append(secOpts, "label=disable")
			}

			_, runErr := d.podmanClient.RunContainer(ctx, podman.RunContainerOptions{
				Name:          targetContainerName,
				Image:         imageToRun,
				Ports:         ports,
				Env:           envMap,
				Network:       parsed.Network,
				Labels:        labels,
				RestartPolicy: parsed.RestartPolicy,
				Volumes:       vols,
				UserNS:        parsed.UserNS,
				CapAdd:        parsed.CapAdd,
				CapDrop:       parsed.CapDrop,
				Devices:       parsed.Devices,
				Privileged:    parsed.Privileged,
				SecurityOpt:   secOpts,
			})
			if runErr != nil {
				logLine(fmt.Sprintf("❌ Podman container launch failed: %v", runErr))
				return nil, runErr
			}
			portDesc := "none"
			if len(ports) > 0 {
				portDesc = strings.Join(ports, ", ")
			}
			logLine(fmt.Sprintf("✅ Container '%s' active on network '%s' (ports: %s)", targetContainerName, parsed.Network, portDesc))
		}
	}

	return &DeployResult{
		ContainerName: targetContainerName,
		CommitHash:    "",
		CommitMessage: fmt.Sprintf("Quadlet unit compiled and activated (%s)", imageToRun),
		Image:         imageToRun,
		Version:       "quadlet",
	}, nil
}
