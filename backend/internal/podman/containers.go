package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

// GetContainers returns list of containers.
func (c *Client) GetContainers(ctx context.Context) ([]ContainerItem, error) {
	statsMap := make(map[string]ContainerStat)
	if stats, err := c.GetContainerStats(ctx); err == nil {
		for _, s := range stats {
			statsMap[s.ID] = s
			statsMap[s.Name] = s
		}
	}

	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/containers/json?all=true", nil)
		if err == nil {
			resp, err := c.httpClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var rawList []struct {
					ID      string            `json:"Id"`
					Names   []string          `json:"Names"`
					Image   string            `json:"Image"`
					State   string            `json:"State"`
					Status  string            `json:"Status"`
					Created string            `json:"Created"`
					Labels  map[string]string `json:"Labels"`
					Ports   []struct {
						HostPort      int    `json:"host_port"`
						ContainerPort int    `json:"container_port"`
						Protocol      string `json:"protocol"`
					} `json:"Ports"`
				}

				body, _ := io.ReadAll(resp.Body)
				if err := json.Unmarshal(body, &rawList); err == nil {
					var result []ContainerItem
					for _, cItem := range rawList {
						idShort := cItem.ID
						if len(idShort) > 12 {
							idShort = idShort[:12]
						}

						var ports []string
						for _, p := range cItem.Ports {
							if p.HostPort > 0 {
								ports = append(ports, fmt.Sprintf("%d:%d/%s", p.HostPort, p.ContainerPort, p.Protocol))
							} else {
								ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
							}
						}

						item := ContainerItem{
							ID:      idShort,
							Names:   cItem.Names,
							Image:   cItem.Image,
							Status:  cItem.Status,
							State:   cItem.State,
							Created: cItem.Created,
							Ports:   strings.Join(ports, ", "),
							Labels:  cItem.Labels,
						}

						if s, ok := statsMap[idShort]; ok {
							item.Stats = &s
						} else if len(cItem.Names) > 0 {
							if s, ok := statsMap[strings.TrimPrefix(cItem.Names[0], "/")]; ok {
								item.Stats = &s
							}
						}

						result = append(result, item)
					}
					return result, nil
				}
			}
		}
	}

	return c.getContainersCLI(ctx, statsMap)
}

func (c *Client) getContainersCLI(ctx context.Context, statsMap map[string]ContainerStat) ([]ContainerItem, error) {
	cmd := exec.CommandContext(ctx, "podman", "ps", "-a", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("podman ps CLI error: %w", err)
	}

	var rawList []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &rawList); err != nil {
		return nil, err
	}

	var result []ContainerItem
	for _, raw := range rawList {
		id := getString(raw, "Id")
		if len(id) > 12 {
			id = id[:12]
		}
		namesList, _ := raw["Names"].([]interface{})
		var names []string
		for _, n := range namesList {
			if s, ok := n.(string); ok {
				names = append(names, s)
			}
		}

		item := ContainerItem{
			ID:      id,
			Names:   names,
			Image:   getString(raw, "Image"),
			Status:  getString(raw, "Status"),
			State:   getString(raw, "State"),
			Created: getString(raw, "Created"),
		}

		if s, ok := statsMap[id]; ok {
			item.Stats = &s
		} else if len(names) > 0 {
			if s, ok := statsMap[names[0]]; ok {
				item.Stats = &s
			}
		}

		result = append(result, item)
	}

	return result, nil
}

// GetContainerStats retrieves current resource metrics for containers.
func (c *Client) GetContainerStats(ctx context.Context) ([]ContainerStat, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/containers/stats?stream=false", nil)
		if err == nil {
			resp, err := c.httpClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var raw struct {
					Stats []struct {
						ContainerID string  `json:"ContainerID"`
						Name        string  `json:"Name"`
						CPU         float64 `json:"CPU"`
						MemUsage    int64   `json:"MemUsage"`
						MemLimit    int64   `json:"MemLimit"`
						MemPerc     float64 `json:"MemPerc"`
						BlockInput  int64   `json:"BlockInput"`
						BlockOutput int64   `json:"BlockOutput"`
						PIDs        int     `json:"PIDs"`
						Network     map[string]struct {
							RxBytes int64 `json:"RxBytes"`
							TxBytes int64 `json:"TxBytes"`
						} `json:"Network"`
					} `json:"Stats"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil && len(raw.Stats) > 0 {
					var stats []ContainerStat
					for _, s := range raw.Stats {
						var rx, tx int64
						for _, netStat := range s.Network {
							rx += netStat.RxBytes
							tx += netStat.TxBytes
						}

						idShort := s.ContainerID
						if len(idShort) > 12 {
							idShort = idShort[:12]
						}

						memStr := fmt.Sprintf("%s / %s", formatBytes(s.MemUsage), formatBytes(s.MemLimit))
						netStr := fmt.Sprintf("%s / %s", formatBytes(rx), formatBytes(tx))

						stats = append(stats, ContainerStat{
							ID:          idShort,
							Name:        s.Name,
							CPUPercent:  s.CPU,
							MemUsage:    s.MemUsage,
							MemLimit:    s.MemLimit,
							MemPercent:  s.MemPerc,
							MemDisplay:  memStr,
							NetRx:       rx,
							NetTx:       tx,
							NetDisplay:  netStr,
							BlockInput:  s.BlockInput,
							BlockOutput: s.BlockOutput,
							PIDs:        s.PIDs,
						})
					}
					return stats, nil
				}
			}
		}
	}

	return c.getStatsCLI(ctx)
}

func (c *Client) getStatsCLI(ctx context.Context) ([]ContainerStat, error) {
	cmd := exec.CommandContext(ctx, "podman", "stats", "--no-stream", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("podman stats CLI error: %w", err)
	}

	var rawList []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &rawList); err != nil {
		return nil, err
	}

	var result []ContainerStat
	for _, item := range rawList {
		id := getString(item, "id")
		name := getString(item, "name")
		cpuStr := strings.TrimSuffix(getString(item, "cpu_percent"), "%")
		cpuVal, _ := strconv.ParseFloat(cpuStr, 64)

		memPercStr := strings.TrimSuffix(getString(item, "mem_percent"), "%")
		memPercVal, _ := strconv.ParseFloat(memPercStr, 64)

		pidsStr := getString(item, "pids")
		pidsVal, _ := strconv.Atoi(pidsStr)

		result = append(result, ContainerStat{
			ID:         id,
			Name:       name,
			CPUPercent: cpuVal,
			MemPercent: memPercVal,
			MemDisplay: getString(item, "mem_usage"),
			NetDisplay: getString(item, "net_io"),
			PIDs:       pidsVal,
		})
	}

	return result, nil
}

// RunContainer starts a new container with the specified options.
func (c *Client) RunContainer(ctx context.Context, opts RunContainerOptions) (string, error) {
	args := []string{"run", "-d", "--name", opts.Name}
	for _, p := range opts.Ports {
		args = append(args, "-p", p)
	}
	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	if opts.RestartPolicy != "" {
		args = append(args, "--restart", opts.RestartPolicy)
	}
	if opts.CPULimit > 0 {
		args = append(args, fmt.Sprintf("--cpus=%.2f", opts.CPULimit))
	}
	if opts.MemoryLimit > 0 {
		args = append(args, fmt.Sprintf("--memory=%dm", opts.MemoryLimit))
	}
	args = append(args, opts.Image)

	cmd := exec.CommandContext(ctx, "podman", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman run failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// StartContainer starts a container by ID or name.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.postContainerAction(ctx, id, "start")
}

// StopContainer stops a running container.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.postContainerAction(ctx, id, "stop")
}

// RestartContainer restarts a container.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.postContainerAction(ctx, id, "restart")
}

// DeleteContainer removes a container, optionally forcing removal.
func (c *Client) DeleteContainer(ctx context.Context, id string, force bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/containers/%s?force=%t", id, force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman delete failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman delete failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, id)
	return exec.CommandContext(ctx, "podman", args...).Run()
}

// GetContainerLogs returns logs for a given container.
func (c *Client) GetContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	if tail <= 0 {
		tail = 100
	}

	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/containers/%s/logs?stdout=true&stderr=true&tail=%d", id, tail)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("podman logs request failed: %w", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	cmd := exec.CommandContext(ctx, "podman", "logs", "--tail", fmt.Sprintf("%d", tail), id)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (c *Client) postContainerAction(ctx context.Context, id, action string) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/containers/%s/%s", id, action)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman action %s failed: %w", action, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman %s failed (HTTP %d): %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return exec.CommandContext(ctx, "podman", action, id).Run()
}
