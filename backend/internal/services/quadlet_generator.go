package services

import (
	"fmt"
	"strings"
)

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
	targetContainerName := SanitizeQuadletSlug(s.ID)
	if targetContainerName == "" {
		targetContainerName = cleanName
	}

	if s.ComposeYaml != "" {
		allSvcs := ExtractAllComposeServices(s.ComposeYaml)
		if len(allSvcs) > 1 {
			var chunks []string
			podName := fmt.Sprintf("pod_%s", cleanName)
			var podPublishes strings.Builder
			for _, svc := range allSvcs {
				for _, p := range svc.Ports {
					podPublishes.WriteString(fmt.Sprintf("PublishPort=%s\n", p))
				}
			}

			// 1. Pod unit
			podUnit := fmt.Sprintf("# File: %s.pod\n[Unit]\nDescription=GoPod %s Pod\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Pod]\nPodName=%s\nNetwork=gopod-net\n%s[Install]\nWantedBy=default.target\n",
				podName, cleanName, podName, podPublishes.String())
			chunks = append(chunks, podUnit)

			// 2. Member containers
			for _, svc := range allSvcs {
				cImg := svc.Image
				if cImg == "" {
					cImg = "docker.io/library/nginx:alpine"
				}
				var cExtras strings.Builder
				if svc.Exec != "" {
					cExtras.WriteString(fmt.Sprintf("Exec=%s\n", svc.Exec))
				}
				for _, env := range svc.Environment {
					cExtras.WriteString(fmt.Sprintf("Environment=%s\n", env))
				}
				for _, vol := range svc.Volumes {
					if !strings.Contains(vol, ":") {
						cExtras.WriteString(fmt.Sprintf("Volume=%s:%s:Z\n", SanitizeQuadletSlug(vol), vol))
					} else {
						if !strings.HasSuffix(vol, ":Z") && !strings.HasSuffix(vol, ":z") {
							cExtras.WriteString(fmt.Sprintf("Volume=%s:Z\n", vol))
						} else {
							cExtras.WriteString(fmt.Sprintf("Volume=%s\n", vol))
						}
					}
				}
				rPolicy := svc.Restart
				if rPolicy == "" {
					rPolicy = "always"
				}

				mUnit := fmt.Sprintf("# File: %s.container\n[Unit]\nDescription=GoPod Pod Member %s\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Container]\nContainerName=%s\nImage=%s\nPod=%s.pod\nLabel=io.gopod.service=%s\nLabel=io.gopod.project=%s\n%s[Service]\nRestart=%s\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target\n",
					svc.ContainerName, svc.ContainerName, svc.ContainerName, cImg, podName, s.ID, s.ProjectID, cExtras.String(), rPolicy)
				chunks = append(chunks, mUnit)
			}
			return strings.Join(chunks, "\n---\n")
		}
	}

	var extracted ExtractedCompose
	if s.ComposeYaml != "" {
		extracted = ExtractComposeWorkload(s.ComposeYaml)
		if extracted.ContainerName != "" {
			targetContainerName = extracted.ContainerName
		}
	}

	// Resolve image safely
	image := s.Image
	if extracted.Image != "" {
		image = extracted.Image
	} else if err := ValidateContainerImage(image); err != nil {
		if s.ComposeYaml != "" {
			extImg := ExtractFirstImageFromCompose(s.ComposeYaml)
			if extImg != "" && ValidateContainerImage(extImg) == nil {
				image = extImg
			}
		}
		if image == "" || ValidateContainerImage(image) != nil {
			image = "docker.io/library/nginx:alpine"
		}
	}

	var publishDirectives strings.Builder
	if len(extracted.Ports) > 0 {
		for _, p := range extracted.Ports {
			publishDirectives.WriteString(fmt.Sprintf("PublishPort=%s\n", p))
		}
	} else if s.Type != "compose" && s.ComposeYaml == "" {
		// Only non-compose services fall back to service.hostPort/port
		if s.HostPort > 0 && s.Port > 0 {
			publishDirectives.WriteString(fmt.Sprintf("PublishPort=%d:%d\n", s.HostPort, s.Port))
		} else if s.HostPort > 0 {
			publishDirectives.WriteString(fmt.Sprintf("PublishPort=%d:80\n", s.HostPort))
		}
	}

	var extraDirectives strings.Builder
	if extracted.Exec != "" {
		extraDirectives.WriteString(fmt.Sprintf("Exec=%s\n", extracted.Exec))
	}
	for _, env := range extracted.Environment {
		extraDirectives.WriteString(fmt.Sprintf("Environment=%s\n", env))
	}
	for _, env := range s.EnvVars {
		if env.Key != "" && !strings.HasPrefix(env.Key, "#") {
			extraDirectives.WriteString(fmt.Sprintf("Environment=\"%s=%s\"\n", env.Key, env.Value))
		}
	}
	for _, vol := range extracted.Volumes {
		if !strings.Contains(vol, ":") {
			extraDirectives.WriteString(fmt.Sprintf("Volume=%s:%s:Z\n", SanitizeQuadletSlug(vol), vol))
		} else {
			if !strings.HasSuffix(vol, ":Z") && !strings.HasSuffix(vol, ":z") {
				extraDirectives.WriteString(fmt.Sprintf("Volume=%s:Z\n", vol))
			} else {
				extraDirectives.WriteString(fmt.Sprintf("Volume=%s\n", vol))
			}
		}
	}

	if s.Advanced != nil {
		if s.Advanced.Runtime.UserNamespace != "" && s.Advanced.Runtime.UserNamespace != "default" && s.Advanced.Runtime.UserNamespace != "none" {
			extraDirectives.WriteString(fmt.Sprintf("UserNS=%s\n", s.Advanced.Runtime.UserNamespace))
		}
		for _, v := range s.Advanced.Storage.Volumes {
			if v.Source != "" && v.Target != "" {
				volSource := SanitizeQuadletSlug(v.Source)
				if v.Options != "" {
					extraDirectives.WriteString(fmt.Sprintf("Volume=%s:%s:%s\n", volSource, v.Target, v.Options))
				} else {
					extraDirectives.WriteString(fmt.Sprintf("Volume=%s:%s:Z\n", volSource, v.Target))
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

	restartPolicy := MapDockerRestartToSystemd(s.RestartPolicy)
	if extracted.Restart != "" {
		restartPolicy = MapDockerRestartToSystemd(extracted.Restart)
	}
	if restartPolicy == "" {
		restartPolicy = "always"
	}

	return fmt.Sprintf("[Unit]\nDescription=%s Quadlet Service\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Container]\nContainerName=%s\nImage=%s\nNetwork=gopod-net\nLabel=io.gopod.service=%s\nLabel=io.gopod.project=%s\n%s%s\n[Service]\nRestart=%s\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target\n",
		cleanName, targetContainerName, image, s.ID, s.ProjectID, publishDirectives.String(), extraDirectives.String(), restartPolicy)
}

func (d *Deployer) prepareSingleQuadletContent(s Service, cleanName string, rawContent string) string {
	targetContainerName := SanitizeQuadletSlug(s.ID)
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

		// Sanitize ContainerName if already present to guarantee valid systemd/Podman identifier
		if inContainer && strings.HasPrefix(trimmed, "ContainerName=") {
			currName := strings.TrimSpace(strings.TrimPrefix(trimmed, "ContainerName="))
			cleanCurrName := SanitizeQuadletSlug(currName)
			if cleanCurrName != "" {
				newLines = append(newLines, fmt.Sprintf("ContainerName=%s", cleanCurrName))
			} else {
				newLines = append(newLines, fmt.Sprintf("ContainerName=%s", targetContainerName))
			}
			continue
		}

		// Validate and heal Image reference if it contains spaces or corrupt values
		if inContainer && strings.HasPrefix(trimmed, "Image=") {
			rawImg := strings.TrimSpace(strings.TrimPrefix(trimmed, "Image="))
			if err := ValidateContainerImage(rawImg); err != nil {
				// Fallback: recover image from Compose YAML if available
				if s.ComposeYaml != "" {
					extImg := ExtractComposeWorkload(s.ComposeYaml).Image
					if extImg != "" && ValidateContainerImage(extImg) == nil {
						rawImg = extImg
					}
				}
				// If still has whitespace, replace spaces with dashes or fallback to valid OCI image
				if strings.ContainsAny(rawImg, " \t\r\n") {
					rawImg = strings.ReplaceAll(rawImg, " ", "-")
				}
				if err := ValidateContainerImage(rawImg); err != nil {
					rawImg = "docker.io/library/nginx:alpine"
				}
			}
			newLines = append(newLines, fmt.Sprintf("Image=%s", rawImg))
			continue
		}

		// Podman Quadlet: Restart= is invalid in [Container], belongs in [Service]
		if inContainer && strings.HasPrefix(trimmed, "Restart=") {
			containerRestart = strings.TrimPrefix(trimmed, "Restart=")
			continue
		}

		// Sanitize invalid systemd Restart= directives (e.g. unless-stopped -> always)
		if rVal, ok := strings.CutPrefix(trimmed, "Restart="); ok {
			newLines = append(newLines, fmt.Sprintf("Restart=%s", MapDockerRestartToSystemd(rVal)))
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
			if !strings.Contains(rawContent, "Exec=") && s.ComposeYaml != "" {
				extracted := ExtractComposeWorkload(s.ComposeYaml)
				if extracted.Exec != "" {
					newLines = append(newLines, fmt.Sprintf("Exec=%s", extracted.Exec))
				}
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
		newLines = append(newLines, "", "[Service]", fmt.Sprintf("Restart=%s", MapDockerRestartToSystemd(containerRestart)))
	}

	return strings.Join(newLines, "\n")
}
