package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// deployGit clones repository, builds image with Podman, and starts container.
func (d *Deployer) deployGit(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
	containerName := s.ID
	if containerName == "" {
		containerName = GenerateContainerName(s.ProjectID, s.Name)
	}
	repoUrl := s.GitRepo
	if repoUrl == "" {
		repoUrl = s.Source
	}
	branch := s.GitBranch
	if branch == "" {
		branch = s.Branch
	}
	if branch == "" {
		branch = "main"
	}

	tmpDir := filepath.Join(os.TempDir(), "gopod-builds", fmt.Sprintf("%s-%s", s.ID, depID))
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create build temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Prepare SSH deploy key if configured
	var sshKeyFile string
	if s.SSHKeyID != "" && d.sshKeyProvider != nil {
		privKey, err := d.sshKeyProvider.GetPrivateKey(ctx, s.SSHKeyID)
		if err == nil && strings.TrimSpace(privKey) != "" {
			keyPath := filepath.Join(tmpDir, "deploy_id_ed25519")
			if err := os.WriteFile(keyPath, []byte(strings.TrimSpace(privKey)+"\n"), 0600); err == nil {
				sshKeyFile = keyPath
			}
		}
	}

	logLine(fmt.Sprintf("📥 Cloning %s (branch: %s)...", repoUrl, branch))
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, repoUrl, tmpDir)
	cloneEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if sshKeyFile != "" {
		cloneEnv = append(cloneEnv, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", sshKeyFile))
	}
	cloneCmd.Env = cloneEnv

	stdout, _ := cloneCmd.StdoutPipe()
	stderr, _ := cloneCmd.StderrPipe()
	if err := cloneCmd.Start(); err != nil {
		logLine(fmt.Sprintf("❌ Git clone start failed: %v", err))
		return nil, err
	}
	d.streamPipes(stdout, stderr, logLine)
	if err := cloneCmd.Wait(); err != nil {
		logLine(fmt.Sprintf("❌ Git clone failed: %v", err))
		return nil, err
	}

	// Extract real Git commit SHA, message, and author
	commitHash := ""
	commitMsg := ""
	if out, err := exec.CommandContext(ctx, "git", "-C", tmpDir, "rev-parse", "--short", "HEAD").Output(); err == nil {
		commitHash = strings.TrimSpace(string(out))
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", tmpDir, "log", "-1", "--pretty=%s").Output(); err == nil {
		commitMsg = strings.TrimSpace(string(out))
	}
	logLine(fmt.Sprintf("📌 Git HEAD: %s (%s)", commitHash, commitMsg))

	dockerfile := s.DockerfilePath
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	imageTag := fmt.Sprintf("gopod-%s-%s:latest", strings.ToLower(s.ProjectID), strings.ToLower(s.Name))
	logLine(fmt.Sprintf("🔨 Building container image: %s using %s...", imageTag, dockerfile))

	buildCmd := exec.CommandContext(ctx, "podman", "build", "-t", imageTag, "-f", filepath.Join(tmpDir, dockerfile), tmpDir)
	d.setupCmdEnv(buildCmd)
	bStdout, _ := buildCmd.StdoutPipe()
	bStderr, _ := buildCmd.StderrPipe()
	if err := buildCmd.Start(); err != nil {
		logLine(fmt.Sprintf("❌ Podman build start error: %v", err))
		return nil, err
	}
	d.streamPipes(bStdout, bStderr, logLine)
	if err := buildCmd.Wait(); err != nil {
		logLine(fmt.Sprintf("❌ Podman build failed: %v", err))
		return nil, err
	}
	logLine(fmt.Sprintf("✅ Built image %s successfully", imageTag))

	// Stop previous container right before starting new one
	d.stopPreviousContainer(ctx, s, logLine)

	if err := d.runContainer(ctx, s, containerName, imageTag, logLine); err != nil {
		return nil, err
	}

	return &DeployResult{
		ContainerName: containerName,
		CommitHash:    commitHash,
		CommitMessage: commitMsg,
		Image:         imageTag,
		Version:       commitHash,
	}, nil
}
