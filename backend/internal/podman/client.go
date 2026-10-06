package podman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Client handles communication with Podman via Unix socket or CLI.
type Client struct {
	socketPath string
	httpClient *http.Client
}

// SystemInfo represents host & engine metrics.
type SystemInfo struct {
	Hostname        string  `json:"hostname"`
	OS              string  `json:"os"`
	Kernel          string  `json:"kernel"`
	VCPU            int     `json:"vcpu"`
	CPUUsage        float64 `json:"cpuUsage"`
	MemoryTotal     int64   `json:"memoryTotal"`
	MemoryFree      int64   `json:"memoryFree"`
	MemoryAvailable int64   `json:"memoryAvailable"`
	MemoryUsed      int64   `json:"memoryUsed"`
	MemUsedGB       float64 `json:"memoryUsedGB"`
	MemTotalGB      float64 `json:"memoryTotalGB"`
	MemAvailableGB  float64 `json:"memoryAvailableGB"`
	SwapTotal       int64   `json:"swapTotal"`
	SwapFree        int64   `json:"swapFree"`
	SwapUsed        int64   `json:"swapUsed"`
	SwapUsedMB      float64 `json:"swapUsedMB"`
	SwapTotalGB     float64 `json:"swapTotalGB"`
	PodmanVersion   string  `json:"podmanVersion"`
	Rootless        bool    `json:"rootless"`
	RunningCount    int     `json:"runningCount"`
	StoppedCount    int     `json:"stoppedCount"`
	TotalCount      int     `json:"totalCount"`
	Uptime          string  `json:"uptime"`
	CgroupVersion   string  `json:"cgroupVersion"`
}

// ContainerStat represents resource consumption for a container.
type ContainerStat struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemUsage    int64   `json:"memUsage"`
	MemLimit    int64   `json:"memLimit"`
	MemPercent  float64 `json:"memPercent"`
	MemDisplay  string  `json:"memDisplay"`
	NetRx       int64   `json:"netRx"`
	NetTx       int64   `json:"netTx"`
	NetDisplay  string  `json:"netDisplay"`
	BlockInput  int64   `json:"blockInput"`
	BlockOutput int64   `json:"blockOutput"`
	PIDs        int     `json:"pids"`
}

// ContainerItem represents a container listing.
type ContainerItem struct {
	ID      string            `json:"id"`
	Names   []string          `json:"names"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	State   string            `json:"state"`
	Created string            `json:"created"`
	Ports   string            `json:"ports"`
	Labels  map[string]string `json:"labels,omitempty"`
	Stats   *ContainerStat    `json:"stats,omitempty"`
}

// NewClient creates a new Podman client discovering available sockets.
func NewClient(customSocket string) *Client {
	socket := detectSocket(customSocket)
	var httpClient *http.Client

	if socket != "" {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
			DisableKeepAlives: false,
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   8 * time.Second,
		}
	}

	return &Client{
		socketPath: socket,
		httpClient: httpClient,
	}
}

// SocketPath returns active socket path or empty string if CLI only.
func (c *Client) SocketPath() string {
	return c.socketPath
}

func detectSocket(custom string) string {
	if custom != "" {
		trimmed := strings.TrimPrefix(custom, "unix://")
		if _, err := os.Stat(trimmed); err == nil {
			return trimmed
		}
	}

	if envHost := os.Getenv("CONTAINER_HOST"); envHost != "" {
		trimmed := strings.TrimPrefix(envHost, "unix://")
		if _, err := os.Stat(trimmed); err == nil {
			return trimmed
		}
	}

	if envSock := os.Getenv("PODMAN_SOCKET"); envSock != "" {
		trimmed := strings.TrimPrefix(envSock, "unix://")
		if _, err := os.Stat(trimmed); err == nil {
			return trimmed
		}
	}

	uid := os.Getuid()
	candidates := []string{
		fmt.Sprintf("/run/user/%d/podman/podman.sock", uid),
		filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "podman/podman.sock"),
		"/run/podman/podman.sock",
		"/var/run/docker.sock",
	}

	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// HostMemoryInfo represents memory metrics read from /proc/meminfo.
type HostMemoryInfo struct {
	MemTotal     int64
	MemFree      int64
	MemAvailable int64
	Buffers      int64
	Cached       int64
	SReclaimable int64
	SwapTotal    int64
	SwapFree     int64
}

func readHostMemInfo() (*HostMemoryInfo, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	info := &HostMemoryInfo{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		valBytes := val * 1024
		switch key {
		case "MemTotal":
			info.MemTotal = valBytes
		case "MemFree":
			info.MemFree = valBytes
		case "MemAvailable":
			info.MemAvailable = valBytes
		case "Buffers":
			info.Buffers = valBytes
		case "Cached":
			info.Cached = valBytes
		case "SReclaimable":
			info.SReclaimable = valBytes
		case "SwapTotal":
			info.SwapTotal = valBytes
		case "SwapFree":
			info.SwapFree = valBytes
		}
	}
	return info, nil
}

// GetSystemInfo retrieves system & podman info.
func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/info", nil)
		if err == nil {
			resp, err := c.httpClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var raw struct {
					Host struct {
						CPUs           int    `json:"cpus"`
						Hostname       string `json:"hostname"`
						Kernel         string `json:"kernel"`
						MemTotal       int64  `json:"memTotal"`
						MemFree        int64  `json:"memFree"`
						SwapTotal      int64  `json:"swapTotal"`
						SwapFree       int64  `json:"swapFree"`
						CgroupVersion  string `json:"cgroupVersion"`
						Uptime         string `json:"uptime"`
						Security struct {
							Rootless bool `json:"rootless"`
						} `json:"security"`
						CPUUtilization struct {
							UserPercent   float64 `json:"userPercent"`
							SystemPercent float64 `json:"systemPercent"`
						} `json:"cpuUtilization"`
						Distribution struct {
							Distribution string `json:"distribution"`
							Version      string `json:"version"`
						} `json:"distribution"`
					} `json:"host"`
					Store struct {
						ContainerStore struct {
							Number  int `json:"number"`
							Running int `json:"running"`
							Stopped int `json:"stopped"`
						} `json:"containerStore"`
					} `json:"store"`
					Version struct {
						Version string `json:"version"`
					} `json:"version"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil {
					memTotal := raw.Host.MemTotal
					memFree := raw.Host.MemFree
					var memAvailable int64
					swapTotal := raw.Host.SwapTotal
					swapFree := raw.Host.SwapFree

					if memInfo, err := readHostMemInfo(); err == nil && memInfo.MemTotal > 0 {
						memTotal = memInfo.MemTotal
						memFree = memInfo.MemFree
						memAvailable = memInfo.MemAvailable
						if memAvailable <= 0 {
							memAvailable = memInfo.MemFree + memInfo.Buffers + memInfo.Cached + memInfo.SReclaimable
						}
						if memInfo.SwapTotal > 0 {
							swapTotal = memInfo.SwapTotal
							swapFree = memInfo.SwapFree
						}
					} else {
						memAvailable = memFree
					}

					memUsed := memTotal - memAvailable
					if memUsed < 0 {
						memUsed = 0
					}
					swapUsed := swapTotal - swapFree
					if swapUsed < 0 {
						swapUsed = 0
					}

					cpuUsed := raw.Host.CPUUtilization.UserPercent + raw.Host.CPUUtilization.SystemPercent

					osName := raw.Host.Distribution.Distribution
					if raw.Host.Distribution.Version != "" {
						osName += " " + raw.Host.Distribution.Version
					}

					return &SystemInfo{
						Hostname:        raw.Host.Hostname,
						OS:              osName,
						Kernel:          raw.Host.Kernel,
						VCPU:            raw.Host.CPUs,
						CPUUsage:        cpuUsed,
						MemoryTotal:     memTotal,
						MemoryFree:      memFree,
						MemoryAvailable: memAvailable,
						MemoryUsed:      memUsed,
						MemUsedGB:       float64(memUsed) / (1024 * 1024 * 1024),
						MemTotalGB:      float64(memTotal) / (1024 * 1024 * 1024),
						MemAvailableGB:  float64(memAvailable) / (1024 * 1024 * 1024),
						SwapTotal:       swapTotal,
						SwapFree:        swapFree,
						SwapUsed:        swapUsed,
						SwapUsedMB:      float64(swapUsed) / (1024 * 1024),
						SwapTotalGB:     float64(swapTotal) / (1024 * 1024 * 1024),
						PodmanVersion:   raw.Version.Version,
						Rootless:        raw.Host.Security.Rootless,
						RunningCount:    raw.Store.ContainerStore.Running,
						StoppedCount:    raw.Store.ContainerStore.Stopped,
						TotalCount:      raw.Store.ContainerStore.Number,
						Uptime:          raw.Host.Uptime,
						CgroupVersion:   raw.Host.CgroupVersion,
					}, nil
				}
			}
		}
	}

	// CLI fallback
	return c.getSystemInfoCLI(ctx)
}

func (c *Client) getSystemInfoCLI(ctx context.Context) (*SystemInfo, error) {
	cmd := exec.CommandContext(ctx, "podman", "info", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("podman info failed: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}

	host, _ := raw["host"].(map[string]interface{})
	store, _ := raw["store"].(map[string]interface{})
	sec, _ := raw["security"].(map[string]interface{})
	ver, _ := raw["version"].(map[string]interface{})

	cpus := int(getFloat(host, "cpus"))
	memTotal := int64(getFloat(host, "memTotal"))
	memFree := int64(getFloat(host, "memFree"))
	var memAvailable int64
	swapTotal := int64(getFloat(host, "swapTotal"))
	swapFree := int64(getFloat(host, "swapFree"))

	if memInfo, err := readHostMemInfo(); err == nil && memInfo.MemTotal > 0 {
		memTotal = memInfo.MemTotal
		memFree = memInfo.MemFree
		memAvailable = memInfo.MemAvailable
		if memAvailable <= 0 {
			memAvailable = memInfo.MemFree + memInfo.Buffers + memInfo.Cached + memInfo.SReclaimable
		}
		if memInfo.SwapTotal > 0 {
			swapTotal = memInfo.SwapTotal
			swapFree = memInfo.SwapFree
		}
	} else {
		memAvailable = memFree
	}

	memUsed := memTotal - memAvailable
	if memUsed < 0 {
		memUsed = 0
	}
	swapUsed := swapTotal - swapFree
	if swapUsed < 0 {
		swapUsed = 0
	}

	cpuUtil, _ := host["cpuUtilization"].(map[string]interface{})
	cpuUsed := getFloat(cpuUtil, "userPercent") + getFloat(cpuUtil, "systemPercent")

	dist, _ := host["distribution"].(map[string]interface{})
	osName := getString(dist, "distribution")
	if v := getString(dist, "version"); v != "" {
		osName += " " + v
	}

	cstore, _ := store["containerStore"].(map[string]interface{})

	return &SystemInfo{
		Hostname:        getString(host, "hostname"),
		OS:              osName,
		Kernel:          getString(host, "kernel"),
		VCPU:            cpus,
		CPUUsage:        cpuUsed,
		MemoryTotal:     memTotal,
		MemoryFree:      memFree,
		MemoryAvailable: memAvailable,
		MemoryUsed:      memUsed,
		MemUsedGB:       float64(memUsed) / (1024 * 1024 * 1024),
		MemTotalGB:      float64(memTotal) / (1024 * 1024 * 1024),
		MemAvailableGB:  float64(memAvailable) / (1024 * 1024 * 1024),
		SwapTotal:       swapTotal,
		SwapFree:        swapFree,
		SwapUsed:        swapUsed,
		SwapUsedMB:      float64(swapUsed) / (1024 * 1024),
		SwapTotalGB:     float64(swapTotal) / (1024 * 1024 * 1024),
		PodmanVersion:   getString(ver, "Version"),
		Rootless:        getBool(sec, "rootless"),
		RunningCount:    int(getFloat(cstore, "running")),
		StoppedCount:    int(getFloat(cstore, "stopped")),
		TotalCount:      int(getFloat(cstore, "number")),
		Uptime:          getString(host, "uptime"),
		CgroupVersion:   getString(host, "cgroupVersion"),
	}, nil
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

	// CLI fallback
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

	// CLI fallback
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

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprint(v)
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case string:
			f, _ := strconv.ParseFloat(n, 64)
			return f
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
