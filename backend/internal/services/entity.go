package services

import (
	"encoding/json"
	"time"
)

// EnvVar represents environment variables with secret hiding capability.
type EnvVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// EnvSecretMask is the placeholder for a secret env value in API responses.
// UpdateService treats it (and an empty value) for secret vars as "keep the
// existing value", so a UI roundtrip can never persist the mask literally.
const EnvSecretMask = "••••••"

// Service represents a workload inside a project.
type Service struct {
	ID              string   `json:"id"`
	ProjectID       string   `json:"projectId"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Status          string   `json:"status"`
	Source          string   `json:"source"`
	Branch          string   `json:"branch"`
	Image           string   `json:"image"`
	Port            int      `json:"port"`
	HostPort        int      `json:"hostPort"`
	HostIP          string   `json:"hostIp,omitempty"`
	CPULimit        float64  `json:"cpuLimit"`
	MemoryLimit     int      `json:"memoryLimit"`
	RestartPolicy   string   `json:"restartPolicy"`
	Description     string   `json:"description,omitempty"`
	QuadletConfig   string   `json:"quadletConfig,omitempty"`
	ComposeYaml     string   `json:"composeYaml,omitempty"`
	K8sYaml         string   `json:"k8sYaml,omitempty"`
	RuntimeTarget   string   `json:"runtimeTarget,omitempty"`
	InPod           bool     `json:"inPod"`
	WebhookToken    string   `json:"-"` // secret: only via the reveal/rotate endpoints
	GitRepo         string   `json:"gitRepo,omitempty"`
	GitBranch       string   `json:"gitBranch,omitempty"`
	DockerfilePath  string   `json:"dockerfilePath,omitempty"`
	SSHKeyID        string   `json:"sshKeyId,omitempty"`
	GitCredentialID string   `json:"gitCredentialId,omitempty"`
	BuildArgs       []string `json:"buildArgs,omitempty"`
	SourceType      string   `json:"sourceType,omitempty"`
	BuildType       string   `json:"buildType,omitempty"`
	BuildPath       string   `json:"buildPath,omitempty"`
	PublishDir      string   `json:"publishDir,omitempty"`
	PodID           string   `json:"podId,omitempty"`
	RegistryID      string   `json:"registryId,omitempty"`
	AutoDeploy      bool     `json:"autoDeploy"`
	WatchPaths      string   `json:"watchPaths,omitempty"`
	WebhookProvider string   `json:"webhookProvider,omitempty"`
	// WebhookSecret is the provider signature secret. It is never serialized;
	// HasWebhookSecret reports whether one is configured.
	WebhookSecret string                 `json:"-"`
	Command       string                 `json:"command,omitempty"`
	Entrypoint    string                 `json:"entrypoint,omitempty"`
	EnvVars       []EnvVar               `json:"envVars"`
	Advanced      *ServiceAdvancedConfig `json:"advanced,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	// HasWebhook reports whether a webhook token exists for this service.
	// The token itself is never serialized (json:"-") and is only handed out
	// by the explicit reveal/rotate endpoints.
	HasWebhook bool `json:"hasWebhook,omitempty"`
	// HasWebhookSecret reports whether a provider signature secret is stored.
	HasWebhookSecret bool `json:"hasWebhookSecret,omitempty"`
	// WebhookSecretError is set when a stored signature secret cannot be
	// decrypted with the current master key. Webhook delivery must fail closed
	// in that state; treating the service as unsigned would silently accept
	// forged pushes.
	WebhookSecretError bool `json:"webhookSecretError,omitempty"`
	// EnvDecryptionError is set when the stored env blob could not be decrypted
	// (for example a wiped/replaced master key). The UI shows a warning instead
	// of silently losing the variables.
	EnvDecryptionError bool `json:"envDecryptionError,omitempty"`
	// Quadlet supervision snapshot (populated on single-service GET only):
	// the primary systemd unit name, the unit file path, and whether the
	// host user manager reports it active. Lets the UI distinguish
	// "supervised by systemd" from "served by a direct container".
	QuadletUnit   string `json:"quadletUnit,omitempty"`
	QuadletFile   string `json:"quadletFile,omitempty"`
	QuadletActive bool   `json:"quadletActive,omitempty"`
}

// VolumeMountConfig defines a container volume mount.
type VolumeMountConfig struct {
	Source  string `json:"source"`
	Target  string `json:"target"`
	Options string `json:"options"`
}

// ServiceAdvancedConfig mirrors frontend advanced configuration.
//
// Mock fields were removed: `runtime.mode`, `lifecycle.quadletEnabled`, and
// `lifecycle.systemdUnitName` were persisted but consumed by nothing (the unit
// name is decided by QuadletUnitName()), and `resources.cpuLimit`/`memoryLimit`
// duplicated the enforced top-level Service fields. Old rows still decode:
// encoding/json ignores unknown fields.
type ServiceAdvancedConfig struct {
	Runtime struct {
		UserNamespace string   `json:"userNamespace"`
		Devices       []string `json:"devices"`
	} `json:"runtime"`
	Lifecycle struct {
		RestartPolicy string `json:"restartPolicy"`
	} `json:"lifecycle"`
	Security struct {
		Privileged      bool     `json:"privileged"`
		SELinuxLabel    string   `json:"selinuxLabel"`
		AppArmorProfile string   `json:"apparmorProfile,omitempty"`
		CapAdd          []string `json:"capAdd"`
		CapDrop         []string `json:"capDrop"`
		NoNewPrivileges bool     `json:"noNewPrivileges"`
	} `json:"security"`
	Storage struct {
		Volumes []VolumeMountConfig `json:"volumes"`
	} `json:"storage"`
	Resources struct {
		PIDsLimit int    `json:"pidsLimit"`
		SwapLimit string `json:"swapLimit"`
	} `json:"resources"`
}

// UnmarshalJSON implements custom JSON unmarshaling to tolerate varied or missing createdAt formats.
func (s *Service) UnmarshalJSON(data []byte) error {
	type Alias Service
	aux := &struct {
		CreatedAtRaw *json.RawMessage `json:"createdAt"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.CreatedAtRaw != nil {
		var str string
		if err := json.Unmarshal(*aux.CreatedAtRaw, &str); err == nil && str != "" {
			if t, err := time.Parse(time.RFC3339, str); err == nil {
				s.CreatedAt = t
			} else if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
				s.CreatedAt = t
			} else if t, err := time.Parse("2006-01-02", str); err == nil {
				s.CreatedAt = t
			}
		}
	}
	return nil
}

// Deployment represents a build/deploy execution record.
type Deployment struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"projectId"`
	ServiceID      string     `json:"serviceId"`
	Number         int        `json:"number"`
	Trigger        string     `json:"trigger"`
	Image          string     `json:"image,omitempty"`
	Version        string     `json:"version"`
	CommitHash     string     `json:"commitHash"`
	CommitMessage  string     `json:"commitMessage"`
	Status         string     `json:"status"`
	Duration       string     `json:"duration"`
	ConfigSnapshot string     `json:"configSnapshot,omitempty"`
	StartedAt      time.Time  `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
}
