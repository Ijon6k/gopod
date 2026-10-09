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
	"strings"
	"sync"

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

	// ── 1. KUBERNETES MANIFEST WORKLOAD ──
	if s.Type == "kubernetes" || s.Type == "k8s" || (s.Type != "compose" && s.Type != "quadlet" && strings.TrimSpace(s.K8sYaml) != "") {
		return d.deployKube(ctx, s, depID, trig, logLine)
	}

	// ── 2. COMPOSE WORKLOAD ──
	if s.Type == "compose" || (s.Type != "quadlet" && strings.TrimSpace(s.ComposeYaml) != "") {
		return d.deployCompose(ctx, s, depID, trig, logLine)
	}

	// ── 3. QUADLET WORKLOAD ──
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		return d.deployQuadlet(ctx, s, depID, logLine)
	}

	// ── 4. GIT WORKLOAD ──
	isGit := s.GitRepo != "" || s.Source == "git" ||
		strings.HasPrefix(s.Source, "http://") || strings.HasPrefix(s.Source, "https://") ||
		strings.HasPrefix(s.Source, "git@") || strings.HasSuffix(s.Source, ".git")

	if isGit {
		return d.deployGit(ctx, s, depID, logLine)
	}

	// ── 5. DIRECT IMAGE WORKLOAD ──
	return d.deployImage(ctx, s, depID, logLine)
}

func (d *Deployer) streamPipes(stdout, stderr io.Reader, logLine func(string)) []string {
	var errLines []string
	var mu sync.Mutex
	done := make(chan bool, 2)
	stream := func(r io.Reader, isErr bool) {
		if r == nil {
			done <- true
			return
		}
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			text := scanner.Text()
			logLine(text)
			lower := strings.ToLower(text)
			if isErr || strings.HasPrefix(text, "Error:") ||
				strings.Contains(lower, "requested access to the resource is denied") ||
				strings.Contains(lower, "cannot be used as a dependency") ||
				strings.Contains(lower, "no such container") ||
				strings.Contains(lower, "failed to start") {
				if !strings.Contains(lower, "warning") && !strings.Contains(lower, "resolving") {
					mu.Lock()
					errLines = append(errLines, text)
					mu.Unlock()
				}
			}
		}
		done <- true
	}
	go stream(stdout, false)
	go stream(stderr, true)
	<-done
	<-done
	return errLines
}
