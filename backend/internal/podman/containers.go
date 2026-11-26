package podman

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ErrNotFound reports that the requested podman object is absent from the
// daemon store; callers map it to user-facing "already gone" messaging.
var ErrNotFound = errors.New("podman object not found")

// GetContainers returns list of containers without blocking on heavy stats.
func (c *Client) GetContainers(ctx context.Context) ([]ContainerItem, error) {
	return c.GetContainersWithStats(ctx, false)
}

// GetContainersWithStats returns list of containers with optional live stats.
func (c *Client) GetContainersWithStats(ctx context.Context, includeStats bool) ([]ContainerItem, error) {
	statsMap := make(map[string]ContainerStat)
	if includeStats {
		if stats, err := c.GetContainerStats(ctx); err == nil {
			for _, s := range stats {
				statsMap[s.ID] = s
				statsMap[s.Name] = s
			}
		}
	}

	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/"+c.libpodVersion()+"/libpod/containers/json?all=true", nil)
		if err == nil {
			resp, cancel, err := c.do(ctx, readTimeout, req)
			if err == nil {
				defer cancel()
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var rawList []struct {
						ID        string            `json:"Id"`
						Names     []string          `json:"Names"`
						Image     string            `json:"Image"`
						State     string            `json:"State"`
						Status    string            `json:"Status"`
						Created   string            `json:"Created"`
						CreatedAt string            `json:"CreatedAt"`
						Labels    map[string]string `json:"Labels"`
						Ports     []struct {
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

							status := cItem.Status
							if status == "" {
								status = cItem.State
							}

							item := ContainerItem{
								ID:        idShort,
								Names:     cItem.Names,
								Image:     cItem.Image,
								Status:    status,
								State:     cItem.State,
								Created:   cItem.Created,
								StartedAt: normalizeTimeRef(cItem.CreatedAt),
								Ports:     strings.Join(ports, ", "),
								Labels:    cItem.Labels,
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
	}

	return c.getContainersCLI(ctx, statsMap)
}

func (c *Client) getContainersCLI(ctx context.Context, statsMap map[string]ContainerStat) ([]ContainerItem, error) {
	cmd := exec.CommandContext(ctx, "podman", "ps", "-a", "--format", "json")
	c.SetupCmdEnv(cmd)
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

		labelsMap := make(map[string]string)
		if rawLabels, ok := raw["Labels"].(map[string]interface{}); ok {
			for k, v := range rawLabels {
				if s, ok := v.(string); ok {
					labelsMap[k] = s
				}
			}
		}

		startedAt := getString(raw, "StartedAt")
		if startedAt == "" {
			startedAt = getString(raw, "Created")
		}

		item := ContainerItem{
			ID:        id,
			Names:     names,
			Image:     getString(raw, "Image"),
			Status:    getString(raw, "Status"),
			State:     getString(raw, "State"),
			Created:   getString(raw, "Created"),
			StartedAt: normalizeTimeRef(startedAt),
			Labels:    labelsMap,
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

// normalizeTimeRef normalizes the podman time representations (RFC3339, human
// Go time, unix seconds) into an RFC3339 UTC string. Empty or zero times
// become "" so the UI can fall back honestly instead of showing fake data.
func normalizeTimeRef(raw string) string {
	clean := strings.TrimSpace(raw)
	// CLI JSON may carry numbers as float strings — accept "1752887630.9".
	if unix, err := strconv.ParseFloat(clean, 64); err == nil && unix > 0 {
		return time.Unix(int64(unix), 0).UTC().Format(time.RFC3339)
	}
	if clean == "" || strings.Contains(clean, "0001-01-01T00:00:00Z") {
		return ""
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, clean); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// GetContainerStats retrieves current resource metrics for containers.
func (c *Client) GetContainerStats(ctx context.Context) ([]ContainerStat, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/"+c.libpodVersion()+"/libpod/containers/stats?stream=false", nil)
		if err == nil {
			resp, cancel, err := c.do(ctx, readTimeout, req)
			if err == nil {
				defer cancel()
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
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
	}

	return c.getStatsCLI(ctx)
}

func (c *Client) getStatsCLI(ctx context.Context) ([]ContainerStat, error) {
	cmd := exec.CommandContext(ctx, "podman", "stats", "--no-stream", "--format", "json")
	c.SetupCmdEnv(cmd)
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
	if opts.Pod != "" {
		args = append(args, "--pod", opts.Pod)
	} else {
		for _, p := range opts.Ports {
			args = append(args, "-p", p)
		}
		if opts.Network != "" {
			args = append(args, "--network", opts.Network)
		}
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
	if opts.PidsLimit > 0 {
		args = append(args, fmt.Sprintf("--pids-limit=%d", opts.PidsLimit))
	}
	if opts.MemorySwap != "" {
		args = append(args, fmt.Sprintf("--memory-swap=%s", opts.MemorySwap))
	}
	if opts.UserNS != "" && opts.UserNS != "default" && opts.UserNS != "none" {
		args = append(args, "--userns", opts.UserNS)
	}
	if opts.Privileged {
		args = append(args, "--privileged")
	}
	for _, cap := range opts.CapAdd {
		if cap != "" {
			args = append(args, "--cap-add", cap)
		}
	}
	for _, cap := range opts.CapDrop {
		if cap != "" {
			args = append(args, "--cap-drop", cap)
		}
	}
	for _, dev := range opts.Devices {
		if dev != "" {
			args = append(args, "--device", dev)
		}
	}
	for _, vol := range opts.Volumes {
		if vol != "" {
			// Engine-level choke point: every deployment route (image,
			// quadlet, compose reachability, restart paths) funnels volume
			// specs through here, so reserved host paths are refused exactly
			// once regardless of where the request originated.
			if err := ValidateRunVolumeSpec(vol); err != nil {
				return "", err
			}
			args = append(args, "-v", vol)
		}
	}
	for _, sec := range opts.SecurityOpt {
		if sec != "" {
			args = append(args, "--security-opt", sec)
		}
	}
	for k, v := range opts.Labels {
		if k != "" {
			args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
		}
	}
	for k, v := range opts.Env {
		if k != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
	}
	img := strings.TrimSpace(opts.Image)
	if img == "" || strings.HasPrefix(img, "-") || strings.ContainsAny(img, " \t\r\n") {
		return "", fmt.Errorf("invalid container image %q", opts.Image)
	}
	args = append(args, "--", img)
	if len(opts.Command) > 0 {
		args = append(args, opts.Command...)
	}

	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
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
		reqURL := fmt.Sprintf("http://d/%s/libpod/containers/%s?force=%t", c.libpodVersion(), url.PathEscape(id), force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, writeTimeout, req)
		if err != nil {
			return fmt.Errorf("podman delete failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		// 404 is expected when a preflight cleanup or a pod cascade already
		// removed the container; teardown stays idempotent (same tolerance as
		// DeletePod and DeleteVolume).
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman delete failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	// `--` before the identifier: a client-supplied id like `--all` must be
	// treated as a container name (and then fail), never as a podman flag.
	args = append(args, "--", id)
	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
	return cmd.Run()
}

// GetContainerLogs returns demultiplexed logs for a given container.
func (c *Client) GetContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	return c.GetContainerLogsWithOptions(ctx, id, tail, false)
}

// GetContainerLogsWithOptions returns demultiplexed logs for a given container,
// optionally requesting RFC3339 timestamps from Podman.
func (c *Client) GetContainerLogsWithOptions(ctx context.Context, id string, tail int, timestamps bool) (string, error) {
	if tail <= 0 {
		tail = 100
	}

	if c.httpClient != nil {
		tsParam := ""
		if timestamps {
			tsParam = "&timestamps=true"
		}
		reqURL := fmt.Sprintf("http://d/%s/libpod/containers/%s/logs?stdout=true&stderr=true&tail=%d%s", c.libpodVersion(), url.PathEscape(id), tail, tsParam)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return "", err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return "", fmt.Errorf("podman logs request failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("container %s: %w", id, ErrNotFound)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("podman logs returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		logs, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return demuxContainerLogs(logs), nil
	}

	args := []string{"logs", "--tail", fmt.Sprintf("%d", tail)}
	if timestamps {
		args = append(args, "--timestamps")
	}
	args = append(args, "--", id)
	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
	out, err := cmd.CombinedOutput()
	return strings.ReplaceAll(string(out), "\r\n", "\n"), err
}

func isMultiplexHeader(hdr []byte) bool {
	return len(hdr) >= 8 && hdr[0] <= 3 && hdr[1] == 0 && hdr[2] == 0 && hdr[3] == 0
}

// demuxContainerLogs strips Docker/Podman 8-byte multiplexed stream headers
// ([streamType, 0, 0, 0, uint32_be(len)]) from non-TTY container log payloads,
// falling back cleanly to raw UTF-8 text when the stream is unframed (Tty: true).
func demuxContainerLogs(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if len(raw) < 8 || !isMultiplexHeader(raw) {
		return strings.ReplaceAll(string(raw), "\r\n", "\n")
	}

	var b strings.Builder
	b.Grow(len(raw))
	pos := 0
	decodedFrames := 0

	for pos+8 <= len(raw) {
		hdr := raw[pos : pos+8]
		if !isMultiplexHeader(hdr) {
			if decodedFrames == 0 {
				return strings.ReplaceAll(string(raw), "\r\n", "\n")
			}
			b.Write(raw[pos:])
			pos = len(raw)
			break
		}
		frameLen := binary.BigEndian.Uint32(hdr[4:8])
		remaining := uint64(len(raw) - pos - 8)
		if uint64(frameLen) > remaining {
			if decodedFrames == 0 {
				return strings.ReplaceAll(string(raw), "\r\n", "\n")
			}
			b.Write(raw[pos+8:])
			pos = len(raw)
			break
		}
		pos += 8
		end := pos + int(frameLen)
		b.Write(raw[pos:end])
		pos = end
		decodedFrames++
	}

	if pos < len(raw) {
		if decodedFrames == 0 {
			return strings.ReplaceAll(string(raw), "\r\n", "\n")
		}
		b.Write(raw[pos:])
	}
	return strings.ReplaceAll(b.String(), "\r\n", "\n")
}

// ContainerInspectState holds live container lifecycle, labels, and HEALTHCHECK state.
type ContainerInspectState struct {
	Status       string
	Running      bool
	ExitCode     int
	HasHealth    bool
	HealthStatus string // "", "starting", "healthy", "unhealthy"
	Labels       map[string]string
}

// InspectContainerState returns live execution and HEALTHCHECK status for idOrName.
func (c *Client) InspectContainerState(ctx context.Context, idOrName string) (*ContainerInspectState, error) {
	if strings.TrimSpace(idOrName) == "" {
		return nil, errors.New("container identifier is required")
	}

	parseInspect := func(body []byte) (*ContainerInspectState, error) {
		var single struct {
			State struct {
				Status   string `json:"Status"`
				Running  bool   `json:"Running"`
				ExitCode int    `json:"ExitCode"`
				Health   *struct {
					Status string `json:"Status"`
				} `json:"Health"`
			} `json:"State"`
			Config *struct {
				Healthcheck interface{}       `json:"Healthcheck"`
				Labels      map[string]string `json:"Labels"`
			} `json:"Config"`
		}
		if err := json.Unmarshal(body, &single); err != nil {
			var arr []json.RawMessage
			if aerr := json.Unmarshal(body, &arr); aerr != nil || len(arr) == 0 {
				return nil, err
			}
			if err := json.Unmarshal(arr[0], &single); err != nil {
				return nil, err
			}
		}
		st := &ContainerInspectState{
			Status:   strings.ToLower(strings.TrimSpace(single.State.Status)),
			Running:  single.State.Running,
			ExitCode: single.State.ExitCode,
		}
		if single.Config != nil {
			st.Labels = single.Config.Labels
			if single.Config.Healthcheck != nil {
				st.HasHealth = true
			}
		}
		if single.State.Health != nil && strings.TrimSpace(single.State.Health.Status) != "" {
			st.HasHealth = true
			st.HealthStatus = strings.ToLower(strings.TrimSpace(single.State.Health.Status))
		}
		return st, nil
	}

	if c.httpClient != nil {
		reqURL := fmt.Sprintf("http://d/%s/libpod/containers/%s/json", c.libpodVersion(), url.PathEscape(idOrName))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return nil, fmt.Errorf("podman inspect request failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("container %s: %w", idOrName, ErrNotFound)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("podman inspect returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return parseInspect(body)
	}

	cmd := exec.CommandContext(ctx, "podman", "inspect", "--format", "json", "--", idOrName)
	c.SetupCmdEnv(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("podman inspect failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return parseInspect(out)
}

func (c *Client) postContainerAction(ctx context.Context, id, action string) error {
	if c.httpClient != nil {
		reqURL := fmt.Sprintf("http://d/%s/libpod/containers/%s/%s", c.libpodVersion(), url.PathEscape(id), action)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, writeTimeout, req)
		if err != nil {
			return fmt.Errorf("podman action %s failed: %w", action, err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman %s failed (HTTP %d): %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	cmd := exec.CommandContext(ctx, "podman", action, "--", id)
	c.SetupCmdEnv(cmd)
	return cmd.Run()
}

// PruneContainers removes stopped and exited containers gopod owns. The
// ownership-label filter applies to both the libpod API request and the CLI
// fallback: stopped containers on the host that carry no io.gopod labels may
// belong to tooling other than gopod and are never touched.
func (c *Client) PruneContainers(ctx context.Context) error {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, pruneFiltersURL("http://d/"+c.libpodVersion()+"/libpod/containers/prune"), nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, pruneTimeout, req)
		if err != nil {
			return fmt.Errorf("podman container prune failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman container prune failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	cmd := exec.CommandContext(ctx, "podman", "container", "prune", "-f", "--filter", "label="+OwnershipLabel)
	c.SetupCmdEnv(cmd)
	return cmd.Run()
}
