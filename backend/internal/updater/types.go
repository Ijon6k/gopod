package updater

import (
	"fmt"
	"regexp"
	"strings"
)

// Build-time metadata populated via -ldflags during release builds.
var (
	Version     = "dev"
	BuildCommit = ""
	BuildDate   = ""
	Channel     = "latest"
)

// Upstream identity is strictly allowlisted to prevent SSRF or supply-chain redirects.
const (
	GitHubOwner   = "ijon6k"
	GitHubRepo    = "gopod"
	GHCRImageBase = "ghcr.io/ijon6k/gopod"
)

var (
	targetTagRe  = regexp.MustCompile(`^(latest|dev|v?[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?)$`)
	digestSha256 = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// ValidateTargetTag strictly checks that a requested update tag conforms to an
// allowlisted release/channel tag (no slashes, flags, registries, or whitespace).
func ValidateTargetTag(raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return "", fmt.Errorf("target tag is required")
	}
	if len(clean) > 64 || strings.HasPrefix(clean, "-") {
		return "", fmt.Errorf("invalid release tag %q", raw)
	}
	if !targetTagRe.MatchString(clean) {
		return "", fmt.Errorf("release tag %q does not match allowed format", raw)
	}
	return clean, nil
}

// ValidateChannel normalizes the update channel to "latest" or "dev".
func ValidateChannel(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "dev") {
		return "dev"
	}
	return "latest"
}

// RuntimeMode identifies how the current GOPOD instance is orchestrated.
type RuntimeMode string

const (
	ModeQuadletPod        RuntimeMode = "quadlet-pod"
	ModeQuadletStandalone RuntimeMode = "quadlet-standalone"
	ModeCompose           RuntimeMode = "compose"
	ModeContainer         RuntimeMode = "container"
	ModeDevLocal          RuntimeMode = "dev-local"
	ModeNative            RuntimeMode = "native"
)

// SelfInspection summarizes runtime topology and update capability.
type SelfInspection struct {
	Mode             RuntimeMode `json:"mode"`
	Containerized    bool        `json:"containerized"`
	ContainerID      string      `json:"containerId,omitempty"`
	ContainerName    string      `json:"containerName,omitempty"`
	RunningImage     string      `json:"runningImage,omitempty"`
	LocalDigest      string      `json:"localDigest,omitempty"`
	PodName          string      `json:"podName,omitempty"`
	UnitName         string      `json:"unitName,omitempty"`
	QuadletDir       string      `json:"quadletDir,omitempty"`
	DbusReachable    bool        `json:"dbusReachable"`
	PodmanSocketOK   bool        `json:"podmanSocketOk"`
	IsDevBuild       bool        `json:"isDevBuild"`
	CanAutoUpdate    bool        `json:"canAutoUpdate"`
	AutoUpdateReason string      `json:"autoUpdateReason,omitempty"`
	ManualHint       string      `json:"manualHint,omitempty"`
	ExecutablePath   string      `json:"executablePath,omitempty"`
}

// PodmanCompatibility evaluates host Podman version & safe upgrade hints.
type PodmanCompatibility struct {
	Version               string `json:"version"`
	RecommendedMinVersion string `json:"recommendedMinVersion"`
	Compatible            bool   `json:"compatible"`
	Rootless              bool   `json:"rootless"`
	OS                    string `json:"os"`
	Warning               string `json:"warning,omitempty"`
	UpgradeHint           string `json:"upgradeHint"`
}

// UpdateStatus persists cross-restart update progress in dataDir.
type UpdateStatus struct {
	State      string `json:"state"` // "idle", "updating", "succeeded", "rolled_back", "failed"
	TargetTag  string `json:"targetTag,omitempty"`
	TargetRef  string `json:"targetRef,omitempty"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
}

// UpdateCheckResponse is returned to the Settings UI & version badge.
type UpdateCheckResponse struct {
	Repository      string              `json:"repository"`
	ImageRepo       string              `json:"imageRepo"`
	CurrentVersion  string              `json:"currentVersion"`
	BuildCommit     string              `json:"buildCommit,omitempty"`
	BuildDate       string              `json:"buildDate,omitempty"`
	Channel         string              `json:"channel"`
	LatestVersion   string              `json:"latestVersion,omitempty"`
	LatestTag       string              `json:"latestTag,omitempty"`
	ReleaseURL      string              `json:"releaseUrl,omitempty"`
	ReleaseNotes    string              `json:"releaseNotes,omitempty"`
	PublishedAt     string              `json:"publishedAt,omitempty"`
	RemoteDigest    string              `json:"remoteDigest,omitempty"`
	UpdateAvailable bool                `json:"updateAvailable"`
	CheckedAt       string              `json:"checkedAt"`
	CheckError      string              `json:"checkError,omitempty"`
	Self            SelfInspection      `json:"self"`
	Podman          PodmanCompatibility `json:"podman"`
	LastStatus      UpdateStatus        `json:"lastStatus"`
}

