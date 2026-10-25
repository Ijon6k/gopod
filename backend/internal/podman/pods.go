package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

// GetPods returns list of pods.
func (c *Client) GetPods(ctx context.Context) ([]PodItem, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/"+c.libpodVersion()+"/libpod/pods/json", nil)
		if err != nil {
			return nil, err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return nil, fmt.Errorf("podman pods request failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("podman pods returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var raw []struct {
			ID         string            `json:"Id"`
			Name       string            `json:"Name"`
			Status     string            `json:"Status"`
			Created    string            `json:"Created"`
			InfraID    string            `json:"InfraId"`
			Labels     map[string]string `json:"Labels"`
			Containers []struct {
				ID    string      `json:"Id"`
				Names interface{} `json:"Names"`
			} `json:"Containers"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}

		var result []PodItem
		for _, p := range raw {
			var cNames []string
			for _, cont := range p.Containers {
				switch v := cont.Names.(type) {
				case string:
					if v != "" {
						cNames = append(cNames, strings.TrimPrefix(v, "/"))
					}
				case []interface{}:
					for _, item := range v {
						if str, ok := item.(string); ok && str != "" {
							cNames = append(cNames, strings.TrimPrefix(str, "/"))
						}
					}
				case []string:
					for _, str := range v {
						if str != "" {
							cNames = append(cNames, strings.TrimPrefix(str, "/"))
						}
					}
				}
				if len(cNames) == 0 && cont.ID != "" {
					idShort := cont.ID
					if len(idShort) > 12 {
						idShort = idShort[:12]
					}
					cNames = append(cNames, idShort)
				}
			}
			idShort := p.ID
			if len(idShort) > 12 {
				idShort = idShort[:12]
			}
			result = append(result, PodItem{
				ID:         idShort,
				Name:       p.Name,
				Status:     p.Status,
				Containers: cNames,
				Created:    p.Created,
				InfraID:    p.InfraID,
				Labels:     p.Labels,
			})
		}
		return result, nil
	}

	cmd := exec.CommandContext(ctx, "podman", "pod", "ps", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}

	var pods []PodItem
	for _, item := range raw {
		id := getString(item, "Id")
		if len(id) > 12 {
			id = id[:12]
		}
		pods = append(pods, PodItem{
			ID:      id,
			Name:    getString(item, "Name"),
			Status:  getString(item, "Status"),
			Created: getString(item, "Created"),
			InfraID: getString(item, "InfraId"),
			Labels:  getLabels(item, "Labels"),
		})
	}
	return pods, nil
}

// DeletePod removes a pod, optionally forcing removal.
func (c *Client) DeletePod(ctx context.Context, idOrName string, force bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/%s/libpod/pods/%s?force=%t", c.libpodVersion(), idOrName, force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, writeTimeout, req)
		if err != nil {
			return fmt.Errorf("podman delete pod failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman delete pod failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	args := []string{"pod", "rm"}
	if force {
		args = append(args, "-f")
	}
	// `--` before the identifier: a client-supplied id like `--all` must be
	// treated as a pod name (and then fail), never as a podman flag.
	args = append(args, "--", idOrName)
	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
	return cmd.Run()
}

// CreatePod creates a new Podman pod with port mappings and network.
func (c *Client) CreatePod(ctx context.Context, opts CreatePodOptions) (string, error) {
	if c.httpClient != nil {
		payload := map[string]interface{}{
			"name": opts.Name,
		}
		if len(opts.Labels) > 0 {
			payload["labels"] = opts.Labels
		}
		if opts.Network != "" {
			payload["networks"] = map[string]interface{}{opts.Network: map[string]interface{}{}}
		}
		var portMappings []map[string]interface{}
		for _, p := range opts.Ports {
			hostIP, hp, cp, proto := parsePortMapping(p)
			if hp > 0 && cp > 0 {
				m := map[string]interface{}{
					"host_port":      hp,
					"container_port": cp,
					"protocol":       proto,
				}
				if strings.TrimSpace(hostIP) != "" {
					m["host_ip"] = strings.TrimSpace(hostIP)
				}
				portMappings = append(portMappings, m)
			}
		}
		if len(portMappings) > 0 {
			payload["portmappings"] = portMappings
		}
		b, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://d/"+c.libpodVersion()+"/libpod/pods/create", bytes.NewReader(b))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, cancel, err := c.do(ctx, writeTimeout, req)
			if err == nil {
				defer cancel()
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
					var res struct {
						ID string `json:"Id"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.ID != "" {
						return res.ID, nil
					}
					// The daemon accepted creation but the response carried
					// no usable Id: resolve the pod by name from the store
					// listing. The listing reports CLI-truncated (12-char)
					// ids; callers either ignore the returned id or resolve
					// pods through InspectPod, which matches both widths.
					pods, listErr := c.GetPods(ctx)
					if listErr != nil {
						return "", fmt.Errorf("pod create for %s returned no Id and the pods listing failed: %w", opts.Name, listErr)
					}
					for _, p := range pods {
						if p.Name == opts.Name {
							return p.ID, nil
						}
					}
					return "", fmt.Errorf("pod create succeeded but pod %s is missing from the store listing", opts.Name)
				}
			}
		}
	}

	// CLI fallback
	args := []string{"pod", "create", "--name", opts.Name}
	for _, p := range opts.Ports {
		args = append(args, "-p", p)
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	for k, v := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman pod create failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parsePortMapping normalizes "host:container", "host:container/udp",
// "host:container/tcp", and "ip:host:container[/proto]" into its parts.
// The leading bind IP is preserved ("" when absent, meaning all interfaces)
// so callers can forward it as host_ip; protocol defaults to tcp.
func parsePortMapping(p string) (hostIP string, hostPort, containerPort int, proto string) {
	proto = "tcp"
	clean := strings.TrimSpace(p)
	if strings.HasSuffix(strings.ToLower(clean), "/udp") {
		proto = "udp"
		clean = clean[:len(clean)-4]
	} else if strings.HasSuffix(strings.ToLower(clean), "/tcp") {
		clean = clean[:len(clean)-4]
	}
	parts := strings.Split(clean, ":")
	var hStr, cStr string
	switch {
	case len(parts) == 2:
		hStr, cStr = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	case len(parts) >= 3:
		hostIP = strings.TrimSpace(strings.Join(parts[:len(parts)-2], ":"))
		hStr, cStr = strings.TrimSpace(parts[len(parts)-2]), strings.TrimSpace(parts[len(parts)-1])
	default:
		return "", 0, 0, proto
	}
	_, _ = fmt.Sscanf(hStr, "%d", &hostPort)
	_, _ = fmt.Sscanf(cStr, "%d", &containerPort)
	return hostIP, hostPort, containerPort, proto
}

// StartPod starts a pod by ID or name.
func (c *Client) StartPod(ctx context.Context, idOrName string) error {
	return c.postPodAction(ctx, idOrName, "start")
}

// StopPod stops a running pod.
func (c *Client) StopPod(ctx context.Context, idOrName string) error {
	return c.postPodAction(ctx, idOrName, "stop")
}

// RestartPod restarts a pod.
func (c *Client) RestartPod(ctx context.Context, idOrName string) error {
	return c.postPodAction(ctx, idOrName, "restart")
}

func (c *Client) postPodAction(ctx context.Context, idOrName, action string) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/%s/libpod/pods/%s/%s", c.libpodVersion(), idOrName, action)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, writeTimeout, req)
		if err != nil {
			return fmt.Errorf("podman pod %s failed: %w", action, err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman pod %s returned %d: %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	cmd := exec.CommandContext(ctx, "podman", "pod", action, idOrName)
	c.SetupCmdEnv(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman pod %s error: %s (%w)", action, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// InspectPod checks whether a pod exists by ID or name and returns its item.
// Names match exactly and are checked first; the store listing truncates IDs
// to 12 characters (matching podman CLI output), so a full-ID caller matches
// by prefix comparison in either direction.
func (c *Client) InspectPod(ctx context.Context, idOrName string) (*PodItem, error) {
	pods, err := c.GetPods(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		if p.Name == idOrName {
			return &p, nil
		}
	}
	for _, p := range pods {
		short, long := p.ID, idOrName
		if len(short) > len(long) {
			short, long = long, short
		}
		if len(short) >= 12 && strings.HasPrefix(long, short) {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("pod %s not found", idOrName)
}
