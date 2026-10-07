package services

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopod/internal/podman"
)

// SSHKeyProvider provides private SSH keys for Git authentication.
type SSHKeyProvider interface {
	GetPrivateKey(ctx context.Context, id string) (string, error)
}

// DeployResult holds metadata about a completed deployment.
type DeployResult struct {
	ContainerName string
	CommitHash    string
	CommitMessage string
	Image         string
	Version       string
}

// Deployer manages container lifecycle, rolling updates, compose stacks, and source builds.
type Deployer struct {
	podmanClient   *podman.Client
	sshKeyProvider SSHKeyProvider
	logManager     *LogManager
	dataDir        string
}

// NewDeployer creates a new container deployer.
func NewDeployer(client *podman.Client, keyProvider SSHKeyProvider, logMgr *LogManager, dataDir string) *Deployer {
	if dataDir == "" {
		dataDir = "./data"
	}
	_ = os.MkdirAll(dataDir, 0755)
	return &Deployer{
		podmanClient:   client,
		sshKeyProvider: keyProvider,
		logManager:     logMgr,
		dataDir:        dataDir,
	}
}

// setupCmdEnv configures environment variables so that subprocesses (podman CLI, podman-compose) connect to the host Podman socket.
func (d *Deployer) setupCmdEnv(cmd *exec.Cmd) {
	socket := ""
	if d.podmanClient != nil {
		socket = d.podmanClient.SocketPath()
	}
	if socket == "" {
		socket = os.Getenv("PODMAN_SOCKET")
	}
	if socket == "" {
		socket = os.Getenv("CONTAINER_HOST")
	}
	if socket == "" {
		socket = "/run/podman/podman.sock"
	}
	socketClean := strings.TrimPrefix(socket, "unix://")
	socketURL := fmt.Sprintf("unix://%s", socketClean)

	existingEnv := os.Environ()
	var newEnv []string
	for _, e := range existingEnv {
		if !strings.HasPrefix(e, "CONTAINER_HOST=") &&
			!strings.HasPrefix(e, "DOCKER_HOST=") &&
			!strings.HasPrefix(e, "PODMAN_SOCKET=") {
			newEnv = append(newEnv, e)
		}
	}
	newEnv = append(newEnv,
		fmt.Sprintf("CONTAINER_HOST=%s", socketURL),
		fmt.Sprintf("DOCKER_HOST=%s", socketURL),
		fmt.Sprintf("PODMAN_SOCKET=%s", socketClean),
	)
	cmd.Env = newEnv
}

// GenerateContainerName creates a collision-resistant name: <project>-<service>-<shortHash>
func GenerateContainerName(projectID, serviceName string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	hash := hex.EncodeToString(b)

	cleanProject := strings.ToLower(strings.ReplaceAll(projectID, " ", "-"))
	cleanService := strings.ToLower(strings.ReplaceAll(serviceName, " ", "-"))

	return fmt.Sprintf("%s-%s-%s", cleanProject, cleanService, hash)
}

// DeployService executes a deployment according to the service workload type and writes logs.
func (d *Deployer) DeployService(ctx context.Context, s Service, depID string, trigger ...string) (*DeployResult, error) {
	trig := "manual"
	if len(trigger) > 0 && trigger[0] != "" {
		trig = trigger[0]
	}

	logLine := func(line string) {
		if d.logManager != nil {
			d.logManager.LogLine(depID, line)
		}
		log.Printf("[%s] %s", s.Name, line)
	}

	logLine(fmt.Sprintf("🚀 Starting deployment for service '%s' (type: %s, trigger: %s)", s.Name, s.Type, trig))

	// ── 1. COMPOSE WORKLOAD ──
	if s.Type == "compose" || (s.Type != "quadlet" && strings.TrimSpace(s.ComposeYaml) != "") {
		return d.deployCompose(ctx, s, depID, trig, logLine)
	}

	// ── 2. QUADLET WORKLOAD ──
	if s.Type == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		return d.deployQuadlet(ctx, s, depID, logLine)
	}

	// ── 3. GIT WORKLOAD ──
	isGit := s.GitRepo != "" || s.Source == "git" ||
		strings.HasPrefix(s.Source, "http://") || strings.HasPrefix(s.Source, "https://") ||
		strings.HasPrefix(s.Source, "git@") || strings.HasSuffix(s.Source, ".git")

	if isGit {
		return d.deployGit(ctx, s, depID, logLine)
	}

	// ── 4. DIRECT IMAGE WORKLOAD ──
	return d.deployImage(ctx, s, depID, logLine)
}

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

	d.streamPipes(stdout, stderr, logLine)

	if err := cmd.Wait(); err != nil {
		logLine(fmt.Sprintf("❌ Compose execution exited with error: %v", err))
		return nil, err
	}

	logLine("✅ Compose stack deployed successfully and containers are active.")
	return &DeployResult{
		ContainerName: projectName,
		CommitHash:    "",
		CommitMessage: "Compose stack rollout",
		Version:       "compose",
	}, nil
}

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

// deployImage pulls container image and starts container.
func (d *Deployer) deployImage(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
	imageToRun := s.Image
	if imageToRun == "" {
		imageToRun = "docker.io/library/nginx:alpine"
	}

	logLine(fmt.Sprintf("📦 Pulling container image '%s'...", imageToRun))
	pullCmd := exec.CommandContext(ctx, "podman", "pull", imageToRun)
	d.setupCmdEnv(pullCmd)
	pStdout, _ := pullCmd.StdoutPipe()
	pStderr, _ := pullCmd.StderrPipe()
	if err := pullCmd.Start(); err == nil {
		d.streamPipes(pStdout, pStderr, logLine)
		_ = pullCmd.Wait()
	}

	containerName := s.ID
	if containerName == "" {
		containerName = GenerateContainerName(s.ProjectID, s.Name)
	}
	d.stopPreviousContainer(ctx, s, logLine)

	if err := d.runContainer(ctx, s, containerName, imageToRun, logLine); err != nil {
		return nil, err
	}

	tag := "latest"
	if parts := strings.Split(imageToRun, ":"); len(parts) > 1 {
		tag = parts[len(parts)-1]
	}

	return &DeployResult{
		ContainerName: containerName,
		CommitHash:    "",
		CommitMessage: fmt.Sprintf("Image deploy: %s", imageToRun),
		Image:         imageToRun,
		Version:       tag,
	}, nil
}

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

func (d *Deployer) runContainer(ctx context.Context, s Service, containerName, imageToRun string, logLine func(string)) error {
	ports := []string{}
	if s.Port > 0 {
		// Default map host:container
		ports = append(ports, fmt.Sprintf("%d:%d", s.Port, s.Port))
	}

	envMap := make(map[string]string)
	for _, env := range s.EnvVars {
		if env.Key != "" {
			envMap[env.Key] = env.Value
		}
	}

	labels := map[string]string{
		"io.gopod.project": s.ProjectID,
		"io.gopod.service": s.ID,
		"io.gopod.name":    s.Name,
	}

	logLine(fmt.Sprintf("🚀 Starting container '%s' (image: %s)...", containerName, imageToRun))
	if d.podmanClient != nil {
		_, err := d.podmanClient.RunContainer(ctx, podman.RunContainerOptions{
			Name:          containerName,
			Image:         imageToRun,
			Ports:         ports,
			Env:           envMap,
			Network:       fmt.Sprintf("%s-network", s.ProjectID),
			Labels:        labels,
			RestartPolicy: s.RestartPolicy,
			CPULimit:      s.CPULimit,
			MemoryLimit:   s.MemoryLimit,
		})
		if err != nil {
			logLine(fmt.Sprintf("❌ Container run failed: %v", err))
			return err
		}
	}
	logLine(fmt.Sprintf("✅ Container '%s' is up and healthy.", containerName))
	return nil
}

func (d *Deployer) stopPreviousContainer(ctx context.Context, s Service, logLine func(string)) {
	if d.podmanClient == nil {
		return
	}
	existing, _ := d.podmanClient.GetContainers(ctx)
	for _, c := range existing {
		isSameService := false
		if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID) {
			isSameService = true
		} else if len(c.Names) > 0 && (strings.HasPrefix(c.Names[0], s.ID) || strings.HasPrefix(c.Names[0], fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) || c.Names[0] == s.Name) {
			isSameService = true
		}
		if isSameService {
			logLine(fmt.Sprintf("🔄 Stopping previous container '%s'...", c.ID))
			_ = d.podmanClient.StopContainer(ctx, c.ID)
			_ = d.podmanClient.DeleteContainer(ctx, c.ID, true)
		}
	}
}

func (d *Deployer) streamPipes(stdout, stderr io.Reader, logLine func(string)) {
	done := make(chan bool, 2)
	stream := func(r io.Reader) {
		if r == nil {
			done <- true
			return
		}
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logLine(scanner.Text())
		}
		done <- true
	}
	go stream(stdout)
	go stream(stderr)
	<-done
	<-done
}

// StartService powers on workload containers or units according to service workload type.
func (d *Deployer) StartService(ctx context.Context, s Service) error {
	// 1. Compose Stack
	if s.Type == "compose" || strings.TrimSpace(s.ComposeYaml) != "" {
		projectName := s.ID
		if projectName == "" {
			projectName = s.Name
		}

		composeFile := filepath.Join(d.dataDir, "compose", s.ProjectID, projectName, "docker-compose.yml")
		if _, err := os.Stat(composeFile); err != nil {
			legacyFile := filepath.Join(d.dataDir, "compose", s.ProjectID, s.Name, "docker-compose.yml")
			if _, err2 := os.Stat(legacyFile); err2 == nil {
				composeFile = legacyFile
			} else {
				_, err := d.DeployService(ctx, s, fmt.Sprintf("start-%d", time.Now().Unix()))
				return err
			}
		}

		runner := "podman-compose"
		args := []string{"-p", projectName, "-f", composeFile, "up", "-d"}
		if _, err := exec.LookPath("podman-compose"); err != nil {
			runner = "podman"
			args = []string{"compose", "-p", projectName, "-f", composeFile, "up", "-d"}
		}

		cmd := exec.CommandContext(ctx, runner, args...)
		d.setupCmdEnv(cmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			if runner == "podman-compose" {
				altCmd := exec.CommandContext(ctx, "podman", "compose", "-p", projectName, "-f", composeFile, "up", "-d")
				d.setupCmdEnv(altCmd)
				if altOut, altErr := altCmd.CombinedOutput(); altErr == nil {
					return nil
				} else {
					return fmt.Errorf("compose start error: %s / %s (%w)", strings.TrimSpace(string(out)), strings.TrimSpace(string(altOut)), err)
				}
			}
			return fmt.Errorf("compose start error: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		return nil
	}

	// 2. Quadlet Service
	if s.Type == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd := exec.CommandContext(ctx, "systemctl", "--user", "start", fmt.Sprintf("%s.service", s.Name))
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl start error: %s (%w)", strings.TrimSpace(string(out)), err)
			}
			return nil
		}
	}

	// 3. Container / Git / Direct Image
	if d.podmanClient != nil {
		containers, _ := d.podmanClient.GetContainers(ctx)
		found := false
		for _, c := range containers {
			if (c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID)) ||
				(len(c.Names) > 0 && (strings.HasPrefix(c.Names[0], s.ID) || c.Names[0] == s.Name || strings.HasPrefix(c.Names[0], fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)))) {
				_ = d.podmanClient.StartContainer(ctx, c.ID)
				found = true
			}
		}
		if found {
			return nil
		}
	}

	// Fallback to fresh deployment if no container exists
	_, err := d.DeployService(ctx, s, fmt.Sprintf("start-%d", time.Now().Unix()))
	return err
}

// StopService gracefully halts containers or units for the service workload.
func (d *Deployer) StopService(ctx context.Context, s Service) error {
	// 1. Compose Stack
	if s.Type == "compose" || strings.TrimSpace(s.ComposeYaml) != "" {
		projectName := s.ID
		if projectName == "" {
			projectName = s.Name
		}

		composeFile := filepath.Join(d.dataDir, "compose", s.ProjectID, projectName, "docker-compose.yml")
		if _, err := os.Stat(composeFile); err != nil {
			legacyFile := filepath.Join(d.dataDir, "compose", s.ProjectID, s.Name, "docker-compose.yml")
			if _, err2 := os.Stat(legacyFile); err2 == nil {
				composeFile = legacyFile
			}
		}

		if _, err := os.Stat(composeFile); err == nil {
			runner := "podman-compose"
			args := []string{"-p", projectName, "-f", composeFile, "stop"}
			if _, err := exec.LookPath("podman-compose"); err != nil {
				runner = "podman"
				args = []string{"compose", "-p", projectName, "-f", composeFile, "stop"}
			}
			cmd := exec.CommandContext(ctx, runner, args...)
			d.setupCmdEnv(cmd)
			if out, err := cmd.CombinedOutput(); err != nil {
				if runner == "podman-compose" {
					altCmd := exec.CommandContext(ctx, "podman", "compose", "-p", projectName, "-f", composeFile, "stop")
					d.setupCmdEnv(altCmd)
					if _, altErr := altCmd.CombinedOutput(); altErr == nil {
						return nil
					}
				}
				return fmt.Errorf("compose stop error: %s (%w)", strings.TrimSpace(string(out)), err)
			}
		}
		return nil
	}

	// 2. Quadlet Service
	if s.Type == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd := exec.CommandContext(ctx, "systemctl", "--user", "stop", fmt.Sprintf("%s.service", s.Name))
			_ = cmd.Run()
			return nil
		}
	}

	// 3. Container / Git / Direct Image
	if d.podmanClient != nil {
		containers, _ := d.podmanClient.GetContainers(ctx)
		for _, c := range containers {
			if (c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID)) ||
				(len(c.Names) > 0 && (strings.HasPrefix(c.Names[0], s.ID) || c.Names[0] == s.Name || strings.HasPrefix(c.Names[0], fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)))) {
				_ = d.podmanClient.StopContainer(ctx, c.ID)
			}
		}
	}

	return nil
}

// RestartService restarts running containers or units for the service workload.
func (d *Deployer) RestartService(ctx context.Context, s Service) error {
	_ = d.StopService(ctx, s)
	return d.StartService(ctx, s)
}

