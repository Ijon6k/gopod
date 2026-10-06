package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
}

// Deployer manages container lifecycle, rolling updates, and source builds.
type Deployer struct {
	podmanClient   *podman.Client
	sshKeyProvider SSHKeyProvider
}

// NewDeployer creates a new container deployer with optional SSH key provider.
func NewDeployer(client *podman.Client, keyProvider SSHKeyProvider) *Deployer {
	return &Deployer{
		podmanClient:   client,
		sshKeyProvider: keyProvider,
	}
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

// DeployService performs container creation, rolling update, and returns deployment metadata.
func (d *Deployer) DeployService(ctx context.Context, s Service) (*DeployResult, error) {
	containerName := GenerateContainerName(s.ProjectID, s.Name)
	imageToRun := s.Image
	commitHash := ""
	commitMsg := ""

	// 1. Build from Git repository if git source is configured
	if s.GitRepo != "" || s.Source == "git" {
		log.Printf("🔨 Building service '%s' from Git: %s (branch: %s)", s.Name, s.GitRepo, s.GitBranch)
		builtImage, cHash, cMsg, err := d.buildFromGit(ctx, s, containerName)
		if err != nil {
			log.Printf("⚠️ Git build error: %v", err)
			if s.Image == "" {
				return &DeployResult{ContainerName: containerName}, fmt.Errorf("git build failed: %w", err)
			}
			// Fallback to explicitly specified image if build failed
			imageToRun = s.Image
		} else {
			imageToRun = builtImage
			commitHash = cHash
			commitMsg = cMsg
		}
	}

	if imageToRun == "" {
		return &DeployResult{ContainerName: containerName}, fmt.Errorf("service '%s' has no image or git source specified", s.Name)
	}

	// 2. Stop and remove existing container for this service to avoid port collision
	if d.podmanClient != nil {
		existing, _ := d.podmanClient.GetContainers(ctx)
		for _, c := range existing {
			isSameService := false
			if c.Labels != nil && c.Labels["io.gopod.service"] == s.ID {
				isSameService = true
			} else if len(c.Names) > 0 && (strings.HasPrefix(c.Names[0], fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) || c.Names[0] == s.Name) {
				isSameService = true
			}
			if isSameService {
				log.Printf("🔄 Stopping previous container '%s' for service '%s'", c.ID, s.Name)
				_ = d.podmanClient.StopContainer(ctx, c.ID)
				_ = d.podmanClient.DeleteContainer(ctx, c.ID, true)
			}
		}
	}

	log.Printf("🚀 Deploying service '%s' as container: %s (image: %s)", s.Name, containerName, imageToRun)

	ports := []string{}
	if s.Port > 0 {
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
		log.Printf("⚠️ Podman run notice: %v", err)
		return &DeployResult{ContainerName: containerName, CommitHash: commitHash, CommitMessage: commitMsg}, err
	}

	return &DeployResult{
		ContainerName: containerName,
		CommitHash:    commitHash,
		CommitMessage: commitMsg,
	}, nil
}

// buildFromGit clones the repository and builds a container image with Podman.
// Supports universal Git platforms via private SSH deploy key injection.
func (d *Deployer) buildFromGit(ctx context.Context, s Service, buildTag string) (string, string, string, error) {
	tmpDir := filepath.Join(os.TempDir(), "gopod-builds", buildTag)
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create build temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	branch := s.GitBranch
	if branch == "" {
		branch = "main"
	}

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

	log.Printf("📥 Cloning %s (branch: %s)...", s.GitRepo, branch)
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, s.GitRepo, tmpDir)
	cloneEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if sshKeyFile != "" {
		cloneEnv = append(cloneEnv, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", sshKeyFile))
	}
	cloneCmd.Env = cloneEnv

	if out, err := cloneCmd.CombinedOutput(); err != nil {
		return "", "", "", fmt.Errorf("git clone failed: %s (%w)", string(out), err)
	}

	// Extract real Git commit SHA and commit message
	commitHash := ""
	commitMsg := ""
	if out, err := exec.CommandContext(ctx, "git", "-C", tmpDir, "rev-parse", "--short", "HEAD").Output(); err == nil {
		commitHash = strings.TrimSpace(string(out))
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", tmpDir, "log", "-1", "--pretty=%s").Output(); err == nil {
		commitMsg = strings.TrimSpace(string(out))
	}

	dockerfile := s.DockerfilePath
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	imageTag := fmt.Sprintf("gopod-%s-%s:latest", strings.ToLower(s.ProjectID), strings.ToLower(s.Name))
	log.Printf("📦 Building container image: %s using %s", imageTag, dockerfile)

	buildCmd := exec.CommandContext(ctx, "podman", "build", "-t", imageTag, "-f", filepath.Join(tmpDir, dockerfile), tmpDir)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return "", commitHash, commitMsg, fmt.Errorf("podman build failed: %s (%w)", string(out), err)
	}

	log.Printf("✅ Build finished successfully: %s", imageTag)
	return imageTag, commitHash, commitMsg, nil
}
