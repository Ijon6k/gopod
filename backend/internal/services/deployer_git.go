package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopod/internal/credentials"
)

// ErrDockerfileNotFound marks a build rejected before podman runs.
var ErrDockerfileNotFound = errors.New("dockerfile not found")

// ErrGitCredentialMissing marks a configured HTTPS credential that cannot be loaded.
var ErrGitCredentialMissing = errors.New("git credential not found")

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
	branch := strings.TrimSpace(s.GitBranch)
	if branch == "" {
		branch = strings.TrimSpace(s.Branch)
	}
	if branch == "" {
		branch = "main"
	}
	if err := ValidateGitBranch(branch); err != nil {
		return nil, err
	}

	tmpDir := filepath.Join(os.TempDir(), "gopod-builds", fmt.Sprintf("%s-%s", s.ID, depID))
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create build temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clone credentials live outside the build context. A Dockerfile that does
	// `COPY . .` must never be able to bake an SSH private key or askpass helper
	// into the produced image.
	secretsDir, err := os.MkdirTemp(filepath.Dir(tmpDir), ".git-secrets-")
	if err != nil {
		return nil, fmt.Errorf("failed to create git secret dir: %w", err)
	}
	defer os.RemoveAll(secretsDir)

	cloneURL := repoUrl
	cloneEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var redactions [][2]string

	// HTTPS access token: the username travels in the URL, the secret is
	// supplied by GIT_ASKPASS so it never appears in the process arguments.
	if s.GitCredentialID != "" {
		if !isHTTPSRepoURL(repoUrl) {
			logLine("⚠️ Git access token applies to HTTPS repositories only; continuing with the configured SSH key")
		} else {
			if d.gitTokenProvider == nil {
				return nil, errors.New("git credential provider is not configured")
			}
			cred, err := d.gitTokenProvider.GetGitToken(ctx, s.GitCredentialID)
			if err != nil || cred == nil {
				if err == nil {
					err = ErrGitCredentialMissing
				}
				return nil, fmt.Errorf("load git credential %s: %w", s.GitCredentialID, err)
			}
			askpassPath := filepath.Join(secretsDir, ".git-askpass")
			if err := os.WriteFile(askpassPath, []byte("#!/bin/sh\nprintf '%s\\n' \"$GOPOD_GIT_ASKPASS_TOKEN\"\n"), 0700); err != nil {
				return nil, fmt.Errorf("write git askpass helper: %w", err)
			}
			cloneURL, err = withGitUsername(repoUrl, gitCredentialUsername(cred))
			if err != nil {
				return nil, fmt.Errorf("prepare authenticated clone URL: %w", err)
			}
			cloneEnv = append(cloneEnv,
				"GIT_ASKPASS="+askpassPath,
				"GOPOD_GIT_ASKPASS_TOKEN="+cred.Token,
			)
			redactions = append(redactions, [2]string{cred.Token, "***redacted***"})
			logLine(fmt.Sprintf("🔑 Using Git access token '%s' (%s) for HTTPS clone", cred.Name, cred.Provider))
		}
	}

	// Prepare SSH deploy key if configured. A configured-but-unusable key must
	// fail before cloning: silently dropping it turns a credential problem into
	// an opaque "repository not found" error.
	var sshKeyFile string
	if s.SSHKeyID != "" {
		if d.sshKeyProvider == nil {
			return nil, errors.New("ssh key provider is not configured")
		}
		privKey, err := d.sshKeyProvider.GetPrivateKey(ctx, s.SSHKeyID)
		if err != nil {
			return nil, fmt.Errorf("load ssh key %s: %w", s.SSHKeyID, err)
		}
		if strings.TrimSpace(privKey) == "" {
			return nil, fmt.Errorf("ssh key %s is empty", s.SSHKeyID)
		}
		keyPath := filepath.Join(secretsDir, "deploy_id_ed25519")
		if err := os.WriteFile(keyPath, []byte(strings.TrimSpace(privKey)+"\n"), 0600); err != nil {
			return nil, fmt.Errorf("write ssh key: %w", err)
		}
		sshKeyFile = keyPath
	}

	safeRepoURL := redactCloneURL(repoUrl)
	logLine(fmt.Sprintf("📥 Cloning %s (branch: %s)...", safeRepoURL, branch))
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, "--", cloneURL, tmpDir)
	if sshKeyFile != "" {
		cloneEnv = append(cloneEnv, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", sshKeyFile))
	}
	cloneCmd.Env = cloneEnv

	if safeRepoURL != repoUrl {
		redactions = append(redactions, [2]string{repoUrl, safeRepoURL})
	}
	streamLine := redactedLogger(logLine, redactions)

	stdout, _ := cloneCmd.StdoutPipe()
	stderr, _ := cloneCmd.StderrPipe()
	if err := cloneCmd.Start(); err != nil {
		logLine(fmt.Sprintf("❌ Git clone start failed: %v", err))
		return nil, err
	}
	d.streamPipes(stdout, stderr, streamLine)
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
	result := gitResult(commitHash, commitMsg)

	if buildType := strings.TrimSpace(s.BuildType); buildType != "" && buildType != "dockerfile" {
		logLine(fmt.Sprintf("⚠️ Build type '%s' is not implemented; building with the Dockerfile", buildType))
	}

	dockerfile, err := resolveDockerfile(tmpDir, s.DockerfilePath)
	if err != nil {
		logLine(fmt.Sprintf("❌ %v (HEAD %s)", err, commitHash))
		return result, err
	}

	imageTag := fmt.Sprintf("gopod-%s-%s:latest", strings.ToLower(s.ProjectID), strings.ToLower(s.Name))
	logLine(fmt.Sprintf("🔨 Building container image: %s using %s...", imageTag, dockerfile))

	buildArgs, rejected := normalizeBuildArgs(s.BuildArgs)
	if len(rejected) > 0 {
		logLine(fmt.Sprintf("⚠️ Ignored %d malformed build argument(s); expected KEY=VALUE", len(rejected)))
	}
	buildCmdArgs := []string{"build", "-t", imageTag, "-f", dockerfile}
	for _, arg := range buildArgs {
		buildCmdArgs = append(buildCmdArgs, "--build-arg", arg)
	}
	buildCmdArgs = append(buildCmdArgs, tmpDir)
	buildCmd := exec.CommandContext(ctx, "podman", buildCmdArgs...)
	d.setupCmdEnv(buildCmd)
	bStdout, _ := buildCmd.StdoutPipe()
	bStderr, _ := buildCmd.StderrPipe()
	if err := buildCmd.Start(); err != nil {
		logLine(fmt.Sprintf("❌ Podman build start error: %v", err))
		return result, err
	}
	d.streamPipes(bStdout, bStderr, streamLine)
	if err := buildCmd.Wait(); err != nil {
		logLine(fmt.Sprintf("❌ Podman build failed: %v", err))
		return result, err
	}
	logLine(fmt.Sprintf("✅ Built image %s successfully", imageTag))

	// Pick the redeploy ordering (start-first when safe, stop-then-create
	// otherwise) before reconciling preflight state.
	plan := d.planContainerRedeploy(ctx, s, containerName, logLine)
	if plan.mode == redeployStartFirst {
		d.cleanPreflightForStartFirst(ctx, s, plan.predecessorID, logLine)
	} else {
		d.cleanPreflightConflicts(ctx, s, logLine)
	}

	if err := d.runContainer(ctx, s, plan.runName, imageTag, logLine); err != nil {
		return result, err
	}

	if plan.mode == redeployStartFirst {
		containerName = d.retirePredecessor(ctx, plan.predecessorID, containerName, plan.runName, logLine)
	}

	return &DeployResult{
		ContainerName: containerName,
		CommitHash:    commitHash,
		CommitMessage: commitMsg,
		Image:         imageTag,
		Version:       commitHash,
	}, nil
}

func gitResult(commitHash, commitMsg string) *DeployResult {
	if commitHash == "" && commitMsg == "" {
		return nil
	}
	return &DeployResult{CommitHash: commitHash, CommitMessage: commitMsg, Version: commitHash}
}

func isHTTPSRepoURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && strings.EqualFold(u.Scheme, "https")
}

func withGitUsername(raw, username string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", errors.New("access tokens require an https repository URL")
	}
	if strings.TrimSpace(username) == "" {
		username = "oauth2"
	}
	u.User = url.User(strings.TrimSpace(username))
	return u.String(), nil
}

func gitCredentialUsername(cred *credentials.GitToken) string {
	if username := strings.TrimSpace(cred.Username); username != "" {
		return username
	}
	if strings.EqualFold(cred.Provider, "github") {
		return "x-access-token"
	}
	return "oauth2"
}

func resolveDockerfile(workspace, configured string) (string, error) {
	name := strings.TrimSpace(configured)
	if name == "" {
		name = "Dockerfile"
	}
	clean := filepath.Clean(strings.TrimPrefix(name, "./"))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q escapes the repository", ErrDockerfileNotFound, configured)
	}
	full := filepath.Join(workspace, clean)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("%w: %q", ErrDockerfileNotFound, configured)
	}
	resolvedWorkspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrDockerfileNotFound, configured)
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrDockerfileNotFound, configured)
	}
	rel, err := filepath.Rel(resolvedWorkspace, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q escapes the repository", ErrDockerfileNotFound, configured)
	}
	return full, nil
}

func normalizeBuildArgs(args []string) ([]string, []string) {
	out := make([]string, 0, len(args))
	var rejected []string
	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		if arg == "" {
			continue
		}
		if strings.ContainsAny(arg, "\x00\n\r") {
			rejected = append(rejected, arg)
			continue
		}
		key, _, ok := strings.Cut(arg, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, " \t") {
			rejected = append(rejected, arg)
			continue
		}
		out = append(out, arg)
	}
	return out, rejected
}

func redactedLogger(logLine func(string), pairs [][2]string) func(string) {
	if len(pairs) == 0 {
		return logLine
	}
	return func(line string) {
		for _, pair := range pairs {
			if pair[0] == "" {
				continue
			}
			line = strings.ReplaceAll(line, pair[0], pair[1])
		}
		logLine(line)
	}
}
