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
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/pods/json", nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("podman pods request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("podman pods returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var raw []struct {
			ID         string `json:"Id"`
			Name       string `json:"Name"`
			Status     string `json:"Status"`
			Created    string `json:"Created"`
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
		})
	}
	return pods, nil
}

// DeletePod removes a pod, optionally forcing removal.
func (c *Client) DeletePod(ctx context.Context, idOrName string, force bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/pods/%s?force=%t", idOrName, force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman delete pod failed: %w", err)
		}
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
	args = append(args, idOrName)
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
			parts := strings.Split(p, ":")
			if len(parts) == 2 {
				var hp, cp int
				_, _ = fmt.Sscanf(parts[0], "%d", &hp)
				_, _ = fmt.Sscanf(parts[1], "%d", &cp)
				if hp > 0 && cp > 0 {
					portMappings = append(portMappings, map[string]interface{}{
						"host_port":      hp,
						"container_port": cp,
						"protocol":       "tcp",
					})
				}
			}
		}
		if len(portMappings) > 0 {
			payload["portmappings"] = portMappings
		}
		b, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://d/v5.0.0/libpod/pods/create", bytes.NewReader(b))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
					var res struct {
						ID string `json:"Id"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.ID != "" {
						return res.ID, nil
					}
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
		url := fmt.Sprintf("http://d/v5.0.0/libpod/pods/%s/%s", idOrName, action)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman pod %s failed: %w", action, err)
		}
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
