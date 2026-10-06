package podman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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

// PruneSystem cleans up stopped containers, unused networks, and dangling images.
func (c *Client) PruneSystem(ctx context.Context) error {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://d/v5.0.0/libpod/system/prune", nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman system prune failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman system prune failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return exec.CommandContext(ctx, "podman", "system", "prune", "-f").Run()
}
