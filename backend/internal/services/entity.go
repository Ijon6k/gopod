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

// Service represents a workload inside a project.
type Service struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"projectId"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	Source         string    `json:"source"`
	Branch         string    `json:"branch"`
	Image          string    `json:"image"`
	Port           int       `json:"port"`
	CPULimit       float64   `json:"cpuLimit"`
	MemoryLimit    int       `json:"memoryLimit"`
	RestartPolicy  string    `json:"restartPolicy"`
	Description    string    `json:"description,omitempty"`
	QuadletConfig  string    `json:"quadletConfig,omitempty"`
	ComposeYaml    string    `json:"composeYaml,omitempty"`
	K8sYaml        string    `json:"k8sYaml,omitempty"`
	RuntimeTarget  string    `json:"runtimeTarget,omitempty"`
	WebhookToken   string    `json:"webhookToken,omitempty"`
	GitRepo        string    `json:"gitRepo,omitempty"`
	GitBranch      string    `json:"gitBranch,omitempty"`
	DockerfilePath string    `json:"dockerfilePath,omitempty"`
	SSHKeyID       string    `json:"sshKeyId,omitempty"`
	EnvVars        []EnvVar  `json:"envVars"`
	CreatedAt      time.Time `json:"createdAt"`
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
	ID            string     `json:"id"`
	ProjectID     string     `json:"projectId"`
	ServiceID     string     `json:"serviceId"`
	Number        int        `json:"number"`
	Trigger       string     `json:"trigger"`
	Image         string     `json:"image,omitempty"`
	Version       string     `json:"version"`
	CommitHash    string     `json:"commitHash"`
	CommitMessage string     `json:"commitMessage"`
	Status        string     `json:"status"`
	Duration      string     `json:"duration"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
}
