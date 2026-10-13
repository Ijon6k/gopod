package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// deployDrop deploys an extracted static site directory via an nginx:alpine container.
func (d *Deployer) deployDrop(ctx context.Context, s Service, depID, trigger string, logLine func(string)) (*DeployResult, error) {
	logLine(fmt.Sprintf("🌐 Preparing static site deployment for service '%s'...", s.Name))

	dataDir := d.dataDir
	if dataDir == "" {
		dataDir = "./data"
	}
	distDir := filepath.Join(dataDir, "static", s.ProjectID, s.ID, "dist")

	// Ensure distDir exists; create fallback index.html if empty
	if err := os.MkdirAll(distDir, 0755); err != nil {
		logLine(fmt.Sprintf("❌ Failed to create static directory: %v", err))
		return nil, fmt.Errorf("failed to prepare static directory: %w", err)
	}

	entries, _ := os.ReadDir(distDir)
	if len(entries) == 0 {
		indexPath := filepath.Join(distDir, "index.html")
		_ = os.WriteFile(indexPath, fmt.Appendf(nil, `<!DOCTYPE html>
<html>
<head><title>%s</title><meta charset="utf-8"></head>
<body style="font-family:sans-serif;padding:2rem;text-align:center;">
<h1>GoPod Static Site</h1>
<p>Service: <strong>%s</strong> is ready.</p>
<p>Upload your static assets via the Static Drop tab to publish your website.</p>
</body>
</html>`, s.Name, s.Name), 0644)
	}

	absDistDir, err := filepath.Abs(distDir)
	if err != nil {
		absDistDir = distDir
	}

	logLine(fmt.Sprintf("📂 Serving static directory: %s", absDistDir))

	// Configure container for static nginx hosting
	if s.Image == "" {
		s.Image = "docker.io/library/nginx:alpine"
	}
	if s.Port <= 0 {
		s.Port = 80
	}

	if s.Advanced == nil {
		s.Advanced = &ServiceAdvancedConfig{}
	}

	// Mount the static dist directory to nginx root
	hasDistMount := false
	for _, v := range s.Advanced.Storage.Volumes {
		if v.Target == "/usr/share/nginx/html" {
			hasDistMount = true
			break
		}
	}
	_ = os.Chmod(absDistDir, 0755)
	if !hasDistMount {
		s.Advanced.Storage.Volumes = append(s.Advanced.Storage.Volumes, VolumeMountConfig{
			Source:  absDistDir,
			Target:  "/usr/share/nginx/html",
			Options: "Z",
		})
	}

	res, err := d.deployImage(ctx, s, depID, trigger, logLine)
	if err != nil {
		return nil, err
	}
	res.CommitMessage = fmt.Sprintf("Static drop deploy (%s)", filepath.Base(absDistDir))
	return res, nil
}
