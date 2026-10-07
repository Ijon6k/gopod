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

// deployCompose manages Docker/Podman Compose deployment via CLI with -p project name flag (Dokploy parity).
func (d *Deployer) deployCompose(ctx context.Context, s Service, depID, trigger string, logLine func(string)) (*DeployResult, error) {
	projectName := s.ID
	if projectName == "" {
		projectName = GenerateContainerName(s.ProjectID, s.Name)
	}

	composeDir := filepath.Join(d.dataDir, "compose", s.ProjectID, projectName)
	if err := os.MkdirAll(composeDir, 0755); err != nil {
		logLine(fmt.Sprintf("❌ Failed to create compose workspace dir: %v", err))
		return nil, err
	}

	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	content := strings.TrimSpace(s.ComposeYaml)
	if content == "" {
		content = fmt.Sprintf("version: '3.8'\nservices:\n  %s:\n    image: %s\n    restart: always\n", s.Name, s.Image)
	}

	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		logLine(fmt.Sprintf("❌ Failed to write compose file: %v", err))
		return nil, err
	}

	logLine(fmt.Sprintf("📄 Compose file written to %s (project: %s)", composeFile, projectName))

	if trigger == "fresh-volumes" {
		logLine("🧹 Fresh Volumes requested: purging ephemeral containers and volumes (down -v)...")
		downCmd := exec.CommandContext(ctx, "podman-compose", "-p", projectName, "-f", composeFile, "down", "-v")
		d.setupCmdEnv(downCmd)
		dStdout, _ := downCmd.StdoutPipe()
		dStderr, _ := downCmd.StderrPipe()
		if err := downCmd.Start(); err == nil {
			d.streamPipes(dStdout, dStderr, logLine)
			_ = downCmd.Wait()
		}
	}

	var runner string
	var args []string
	if _, err := exec.LookPath("podman-compose"); err == nil {
		runner = "podman-compose"
		args = []string{"-p", projectName, "-f", composeFile, "up", "-d"}
	} else {
		runner = "podman"
		args = []string{"compose", "-p", projectName, "-f", composeFile, "up", "-d"}
	}

	logLine(fmt.Sprintf("⏳ Executing: %s %s...", runner, strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, runner, args...)
	d.setupCmdEnv(cmd)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		logLine(fmt.Sprintf("⚠️ %s start error: %v. Retrying with alternate runner...", runner, err))
		if runner == "podman-compose" {
			runner = "podman"
			args = []string{"compose", "-p", projectName, "-f", composeFile, "up", "-d"}
		} else {
			runner = "podman-compose"
			args = []string{"-p", projectName, "-f", composeFile, "up", "-d"}
		}
		cmd = exec.CommandContext(ctx, runner, args...)
		d.setupCmdEnv(cmd)
		stdout, _ = cmd.StdoutPipe()
		stderr, _ = cmd.StderrPipe()
		if err2 := cmd.Start(); err2 != nil {
			logLine(fmt.Sprintf("❌ All compose runners failed: %v", err2))
			return nil, err2
		}
	}

	errLines := d.streamPipes(stdout, stderr, logLine)

	if err := cmd.Wait(); err != nil {
		logLine(fmt.Sprintf("❌ Compose execution exited with error: %v", err))
		return nil, err
	}

	// Verify compose outcome: detect if subcontainers failed to pull or start
	hasFatalErrors := false
	var sampleErr string
	for _, l := range errLines {
		lower := strings.ToLower(l)
		if strings.HasPrefix(l, "Error:") ||
			strings.Contains(lower, "requested access to the resource is denied") ||
			strings.Contains(lower, "cannot be used as a dependency") ||
			strings.Contains(lower, "no such container") ||
			strings.Contains(lower, "failed to start") {
			hasFatalErrors = true
			if sampleErr == "" {
				sampleErr = l
			}
		}
	}

	if hasFatalErrors {
		errMsg := fmt.Sprintf("Compose rollout partially or completely failed: %s", sampleErr)
		logLine(fmt.Sprintf("❌ %s", errMsg))
		return nil, errors.New(errMsg)
	}

	logLine("✅ Compose stack deployed successfully and containers are active.")
	return &DeployResult{
		ContainerName: projectName,
		CommitHash:    "",
		CommitMessage: "Compose stack rollout",
		Version:       "compose",
	}, nil
}
