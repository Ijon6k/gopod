package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// deployKube manages Kubernetes Manifest workloads deployed via `podman play kube` (PaaS Parity).
func (d *Deployer) deployKube(ctx context.Context, s Service, depID, trigger string, logLine func(string)) (*DeployResult, error) {
	manifestName := s.ID
	if manifestName == "" {
		manifestName = GenerateContainerName(s.ProjectID, s.Name)
	}

	kubeDir := filepath.Join(d.dataDir, "kube", s.ProjectID, manifestName)
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		logLine(fmt.Sprintf("❌ Failed to create kube workspace directory: %v", err))
		return nil, err
	}

	kubeFile := filepath.Join(kubeDir, "manifest.yaml")
	content := strings.TrimSpace(s.K8sYaml)
	if content == "" {
		content = fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  labels:
    io.gopod.project: "%s"
    io.gopod.service: "%s"
spec:
  containers:
    - name: %s
      image: %s
`, manifestName, s.ProjectID, s.ID, s.Name, s.Image)
	}

	if err := os.WriteFile(kubeFile, []byte(content), 0644); err != nil {
		logLine(fmt.Sprintf("❌ Failed to write Kubernetes manifest file: %v", err))
		return nil, err
	}

	logLine(fmt.Sprintf("📄 Kubernetes manifest written to %s", kubeFile))

	if d.podmanClient != nil {
		_ = d.podmanClient.EnsureNetwork(ctx, "gopod-net")
	}

	args := []string{"play", "kube", "--network", "gopod-net", "--replace"}
	if trigger == "fresh-volumes" {
		args = append(args, "--force")
	}
	args = append(args, kubeFile)

	logLine(fmt.Sprintf("⏳ Executing: podman %s...", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, "podman", args...)
	d.setupCmdEnv(cmd)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		logLine(fmt.Sprintf("❌ podman play kube start failed: %v", err))
		return nil, err
	}

	errLines := d.streamPipes(stdout, stderr, logLine)
	if err := cmd.Wait(); err != nil {
		logLine(fmt.Sprintf("❌ podman play kube exited with error: %v", err))
		return nil, err
	}

	// Verify outcome
	hasFatal := false
	var sampleErr string
	for _, l := range errLines {
		lower := strings.ToLower(l)
		if strings.HasPrefix(l, "Error:") || strings.Contains(lower, "failed to start") {
			hasFatal = true
			if sampleErr == "" {
				sampleErr = l
			}
		}
	}

	if hasFatal {
		errMsg := fmt.Sprintf("Kubernetes deployment failed: %s", sampleErr)
		logLine(fmt.Sprintf("❌ %s", errMsg))
		return nil, errors.New(errMsg)
	}

	logLine("✅ Kubernetes manifest played successfully into Podman runtime.")
	return &DeployResult{
		ContainerName: manifestName,
		CommitHash:    "",
		CommitMessage: "Kubernetes manifest play",
		Version:       "kube",
	}, nil
}
