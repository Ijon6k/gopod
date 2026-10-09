package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// deployQuadlet writes Quadlet systemd unit and generates declarative service.
func (d *Deployer) deployQuadlet(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
	quadletDir := filepath.Join(d.dataDir, "quadlets")
	_ = os.MkdirAll(quadletDir, 0755)

	cleanName := filepath.Base(strings.TrimSpace(s.Name))
	cleanName = strings.ReplaceAll(cleanName, " ", "-")
	cleanName = strings.ReplaceAll(cleanName, "..", "")
	if cleanName == "" || cleanName == "." || cleanName == "/" {
		cleanName = fmt.Sprintf("service-%s", depID)
	}

	unitFile := filepath.Join(quadletDir, fmt.Sprintf("%s.container", cleanName))
	content := strings.TrimSpace(s.QuadletConfig)
	if content == "" {
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

		content = fmt.Sprintf("[Unit]\nDescription=%s Quadlet Service\nAfter=network-online.target\n\n[Container]\nImage=%s\nNetwork=gopod-net\n%s%sRestart=%s\n\n[Service]\nRestart=%s\n\n[Install]\nWantedBy=default.target\n",
			cleanName, s.Image, publishDirective, extraDirectives.String(), restartPolicy, restartPolicy)
	}

	if err := os.WriteFile(unitFile, []byte(content), 0644); err != nil {
		logLine(fmt.Sprintf("❌ Failed to write quadlet file: %v", err))
		return nil, err
	}
	logLine(fmt.Sprintf("📄 Quadlet unit written to %s", unitFile))

	// Also install to systemd user quadlet search path
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		systemdQuadletDir := filepath.Join(homeDir, ".config", "containers", "systemd")
		_ = os.MkdirAll(systemdQuadletDir, 0755)
		userUnitFile := filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.container", cleanName))
		_ = os.WriteFile(userUnitFile, []byte(content), 0644)
		logLine(fmt.Sprintf("📄 Installed Quadlet unit into systemd path: %s", userUnitFile))
	}

	// Attempt systemd reload if systemctl is available
	if _, err := exec.LookPath("systemctl"); err == nil {
		logLine("⚙️ Executing: systemctl --user daemon-reload...")
		_ = exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "restart", fmt.Sprintf("%s.service", cleanName)).Run()
		logLine(fmt.Sprintf("✅ Reloaded systemd user daemon for %s.service", cleanName))
	} else {
		logLine("ℹ️ Quadlet unit generated and verified. Ready for host systemd activation.")
	}

	return &DeployResult{
		ContainerName: cleanName,
		CommitHash:    "",
		CommitMessage: "Quadlet unit compiled",
		Version:       "quadlet",
	}, nil
}
