package podman

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
	SELinuxEnabled  bool    `json:"selinuxEnabled"`
	AppArmorEnabled bool    `json:"apparmorEnabled"`
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

// PodItem represents a Podman pod.
type PodItem struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Containers []string `json:"containers"`
	Network    string   `json:"network"`
	Created    string   `json:"created"`
}

// ImageItem represents a container image in local storage.
type ImageItem struct {
	ID         string `json:"id"`
	Repository string `json:"name"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	Created    string `json:"createdAt"`
}

// VolumeItem represents a local named volume.
type VolumeItem struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	MountPoint string `json:"mount"`
	Size       string `json:"size"`
}

// NetworkItem represents a container network.
type NetworkItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// RunContainerOptions specifies parameters for launching a new container.
type RunContainerOptions struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Ports         []string          `json:"ports,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Network       string            `json:"network,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	RestartPolicy string            `json:"restartPolicy,omitempty"`
	CPULimit      float64           `json:"cpuLimit,omitempty"`
	MemoryLimit   int               `json:"memoryLimit,omitempty"`
	Volumes       []string          `json:"volumes,omitempty"`
	UserNS        string            `json:"userNS,omitempty"`
	Privileged    bool              `json:"privileged,omitempty"`
	CapAdd        []string          `json:"capAdd,omitempty"`
	CapDrop       []string          `json:"capDrop,omitempty"`
	Devices       []string          `json:"devices,omitempty"`
	PidsLimit     int               `json:"pidsLimit,omitempty"`
	SecurityOpt   []string          `json:"securityOpt,omitempty"`
}

// SystemDiskUsage represents disk space usage of images, containers, and volumes.
type SystemDiskUsage struct {
	Type           string `json:"type"`
	Total          int    `json:"total"`
	Active         int    `json:"active"`
	RawSize        int64  `json:"rawSize"`
	RawReclaimable int64  `json:"rawReclaimable"`
	Size           string `json:"size"`
	Reclaimable    string `json:"reclaimable"`
}

