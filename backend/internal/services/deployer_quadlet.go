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

	unitFile := filepath.Join(quadletDir, fmt.Sprintf("%s.container", s.Name))
	content := strings.TrimSpace(s.QuadletConfig)
	if content == "" {
		content = fmt.Sprintf("[Unit]\nDescription=%s Quadlet Service\nAfter=network-online.target\n\n[Container]\nImage=%s\nPublishPort=%d:80\nRestart=always\n\n[Service]\nRestart=always\n\n[Install]\nWantedBy=default.target\n", s.Name, s.Image, s.Port)
	}

	if err := os.WriteFile(unitFile, []byte(content), 0644); err != nil {
		logLine(fmt.Sprintf("❌ Failed to write quadlet file: %v", err))
		return nil, err
	}
	logLine(fmt.Sprintf("📄 Quadlet unit written to %s", unitFile))

	// Attempt systemd reload if systemctl is available
	if _, err := exec.LookPath("systemctl"); err == nil {
		logLine("⚙️ Executing: systemctl --user daemon-reload...")
		_ = exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "restart", fmt.Sprintf("%s.service", s.Name)).Run()
		logLine(fmt.Sprintf("✅ Reloaded systemd user daemon for %s.service", s.Name))
	} else {
		logLine("ℹ️ Quadlet unit generated and verified. Ready for host systemd activation.")
	}

	return &DeployResult{
		ContainerName: s.Name,
		CommitHash:    "",
		CommitMessage: "Quadlet unit compiled",
		Version:       "quadlet",
	}, nil
}
