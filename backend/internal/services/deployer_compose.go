package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// deployCompose manages Docker/Podman Compose deployment via CLI with -p project name flag.
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
	content = injectGopodNetworkToCompose(content)

	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		logLine(fmt.Sprintf("❌ Failed to write compose file: %v", err))
		return nil, err
	}

	logLine(fmt.Sprintf("📄 Compose file written to %s (project: %s)", composeFile, projectName))

	// Write .env file from service environment variables
	envFile := filepath.Join(composeDir, ".env")
	if len(s.EnvVars) > 0 {
		var envBuf strings.Builder
		for _, env := range s.EnvVars {
			if env.Key != "" {
				envBuf.WriteString(fmt.Sprintf("%s=%s\n", env.Key, env.Value))
			}
		}
		if err := os.WriteFile(envFile, []byte(envBuf.String()), 0600); err == nil {
			logLine(fmt.Sprintf("🔑 Environment file written to %s (%d variables)", envFile, len(s.EnvVars)))
		}
	} else {
		_ = os.Remove(envFile)
	}

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

	// Pre-flight ghost container and conflicting systemd unit cleanup
	d.cleanPreflightConflicts(ctx, s, logLine)

	if d.podmanClient != nil {
		_ = d.podmanClient.EnsureNetwork(ctx, "gopod-net")
	}

	var runner string
	var args []string
	usePod := s.InPod || s.RuntimeTarget == "pod"

	if _, err := exec.LookPath("podman-compose"); err == nil {
		runner = "podman-compose"
		podVal := "false"
		if usePod {
			podVal = "true"
		}
		args = []string{"--in-pod", podVal, "-p", projectName, "-f", composeFile, "up", "-d"}
	} else {
		runner = "podman"
		args = []string{"compose", "-p", projectName, "-f", composeFile, "up", "-d"}
	}

	logLine(fmt.Sprintf("⏳ Executing: %s %s...", runner, strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, runner, args...)
	cmd.Dir = composeDir
	d.setupCmdEnv(cmd)
	for _, env := range s.EnvVars {
		if env.Key != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Key, env.Value))
		}
	}

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
		cmd.Dir = composeDir
		d.setupCmdEnv(cmd)
		for _, env := range s.EnvVars {
			if env.Key != "" {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Key, env.Value))
			}
		}
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
		// Explicit fatal errors that must fail the deployment
		if strings.Contains(lower, "bind: address already in use") ||
			strings.Contains(lower, "port already in use") ||
			strings.Contains(lower, "unable to start container") ||
			strings.Contains(lower, "failed to start") ||
			strings.Contains(lower, "requested access to the resource is denied") ||
			strings.Contains(lower, "cannot be used as a dependency") ||
			strings.Contains(lower, "no such container") {
			hasFatalErrors = true
			if sampleErr == "" {
				sampleErr = l
			}
			continue
		}
		// Ignore harmless idempotent warnings like container/pod name collisions during reconciliation
		if strings.Contains(lower, "pod already exists") ||
			strings.Contains(lower, "name is in use") ||
			strings.Contains(lower, "is in use: pod already exists") {
			continue
		}
		if strings.HasPrefix(l, "Error:") {
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

	// Post-rollout verification: ensure all stack member containers entered running state
	if d.podmanClient != nil {
		if containers, err := d.podmanClient.GetContainers(ctx); err == nil {
			var unstarted []string
			for _, c := range containers {
				isMember := false
				if c.Labels != nil && (c.Labels["com.docker.compose.project"] == projectName ||
					c.Labels["io.podman.compose.project"] == projectName ||
					c.Labels["io.gopod.service"] == s.ID) {
					isMember = true
				} else if len(c.Names) > 0 && (strings.HasPrefix(c.Names[0], projectName) || strings.Contains(c.Names[0], s.ID)) {
					isMember = true
				}
				if isMember {
					status := strings.ToLower(c.Status)
					if !strings.HasPrefix(status, "up") && status != "running" && status != "healthy" {
						cName := c.ID[:12]
						if len(c.Names) > 0 {
							cName = c.Names[0]
						}
						unstarted = append(unstarted, fmt.Sprintf("%s (%s)", cName, c.Status))
					}
				}
			}
			if len(unstarted) > 0 {
				errMsg := fmt.Sprintf("Containers failed to enter running state: %s", strings.Join(unstarted, ", "))
				logLine(fmt.Sprintf("❌ %s", errMsg))
				return nil, errors.New(errMsg)
			}
		}
	}

	logLine("✅ Compose stack deployed successfully and containers are active.")

	// Post-rollout: Ensure all member containers are connected to gopod-net
	if d.podmanClient != nil {
		if existing, err := d.podmanClient.GetContainers(ctx); err == nil {
			for _, c := range existing {
				isMember := false
				if c.Labels != nil && (c.Labels["com.docker.compose.project"] == projectName ||
					c.Labels["io.podman.compose.project"] == projectName ||
					c.Labels["io.gopod.service"] == s.ID) {
					isMember = true
				} else if len(c.Names) > 0 && (strings.HasPrefix(c.Names[0], projectName) || strings.Contains(c.Names[0], s.ID)) {
					isMember = true
				}
				if isMember {
					connectArgs := []string{"network", "connect"}
					alias := ""
					if c.Labels != nil {
						if val := c.Labels["com.docker.compose.service"]; val != "" {
							alias = val
						} else if val := c.Labels["io.podman.compose.service"]; val != "" {
							alias = val
						}
					}
					if alias != "" {
						connectArgs = append(connectArgs, "--alias", alias)
					}
					connectArgs = append(connectArgs, "gopod-net", c.ID)
					cmd := exec.CommandContext(ctx, "podman", connectArgs...)
					d.setupCmdEnv(cmd)
					_ = cmd.Run()
				}
			}
		}
	}

	return &DeployResult{
		ContainerName: projectName,
		CommitHash:    "",
		CommitMessage: "Compose stack rollout",
		Version:       "compose",
	}, nil
}

// injectGopodNetworkToCompose safely injects the default gopod-net network into Compose YAML.
// It uses YAML AST parsing to prevent duplicate "networks:" root keys, preserves custom user networks,
// and respects explicit default or gopod-net declarations.
func injectGopodNetworkToCompose(rawYaml string) string {
	raw := strings.TrimSpace(rawYaml)
	if raw == "" {
		return rawYaml
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		// Non-parseable or raw template string; return untouched
		return rawYaml
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return rawYaml
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return rawYaml
	}

	// Helper to create the default: { name: gopod-net, external: true } node pair
	createDefaultEntry := func() (*yaml.Node, *yaml.Node) {
		k := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "default",
		}
		v := &yaml.Node{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "gopod-net"},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "external"},
				{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
			},
		}
		return k, v
	}

	var networksValNode *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "networks" {
			networksValNode = root.Content[i+1]
			break
		}
	}

	if networksValNode != nil {
		// Top-level "networks" key already exists
		if networksValNode.Kind == yaml.MappingNode {
			hasDefault := false
			hasGopodNet := false

			for j := 0; j < len(networksValNode.Content); j += 2 {
				keyName := networksValNode.Content[j].Value
				valChild := networksValNode.Content[j+1]

				if keyName == "default" {
					hasDefault = true
				}
				if keyName == "gopod-net" {
					hasGopodNet = true
				}
				// Also check if any custom network specifies `name: gopod-net`
				if valChild.Kind == yaml.MappingNode {
					for k := 0; k < len(valChild.Content); k += 2 {
						if valChild.Content[k].Value == "name" && valChild.Content[k+1].Value == "gopod-net" {
							hasGopodNet = true
						}
					}
				}
			}

			// If neither default nor gopod-net is configured, merge default into the existing networks block
			if !hasDefault && !hasGopodNet {
				k, v := createDefaultEntry()
				networksValNode.Content = append(networksValNode.Content, k, v)
			}
		} else {
			// networks key is null or empty scalar (e.g. `networks: `)
			k, v := createDefaultEntry()
			networksValNode.Kind = yaml.MappingNode
			networksValNode.Tag = "!!map"
			networksValNode.Content = []*yaml.Node{k, v}
		}
	} else {
		// No "networks" key at root; add top-level networks block
		netKey := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "networks",
		}
		k, v := createDefaultEntry()
		netVal := &yaml.Node{
			Kind:    yaml.MappingNode,
			Tag:     "!!map",
			Content: []*yaml.Node{k, v},
		}
		root.Content = append(root.Content, netKey, netVal)
	}

	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return rawYaml
	}
	_ = enc.Close()

	return buf.String()
}
