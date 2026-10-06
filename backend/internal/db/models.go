package db

import "time"

// Project represents a workspace grouping of services and domains.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
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

// User represents an administrator or team member.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Session represents an authenticated web session.
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// SSHKey represents an SSH deploy key for Git repositories.
type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"publicKey"`
	PrivateKey  string    `json:"privateKey,omitempty"`
	Fingerprint string    `json:"fingerprint"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ContainerRegistry represents credentials for pulling/pushing private container images.
type ContainerRegistry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Username  string    `json:"username"`
	Token     string    `json:"token,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Secret represents a Podman secret.
type Secret struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Value     string    `json:"value,omitempty"`
	Driver    string    `json:"driver"`
	CreatedAt time.Time `json:"createdAt"`
}

// EnvVar represents environment variables with secret hiding capability.
type EnvVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// Domain represents a Caddy reverse proxy routing rule.
type Domain struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"projectId"`
	ServiceID       string    `json:"serviceId"`
	Hostname        string    `json:"hostname"`
	ContainerPort   int       `json:"containerPort"`
	TLS             bool      `json:"tls"`
	HTTPSRedirect   bool      `json:"httpsRedirect"`
	PathPrefix      string    `json:"pathPrefix"`
	StripPathPrefix bool      `json:"stripPathPrefix"`
	WebSocket       bool      `json:"websocket"`
	CORS            bool      `json:"cors"`
	HSTS            bool      `json:"hsts"`
	BasicAuth       bool      `json:"basicAuth"`
	BasicAuthUser   string    `json:"basicAuthUser,omitempty"`
	BasicAuthPass   string    `json:"basicAuthPass,omitempty"`
	DNSStatus       string    `json:"dnsStatus"`
	CreatedAt       time.Time `json:"createdAt"`
}

// VolumeSnapshot represents a .tar.zst archive of a named volume.
type VolumeSnapshot struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	ServiceID   string    `json:"serviceId"`
	VolumeName  string    `json:"volumeName"`
	Filename    string    `json:"filename"`
	Size        string    `json:"size"`
	SizeBytes   int64     `json:"sizeBytes"`
	Status      string    `json:"status"`
	Compression string    `json:"compression"`
	CreatedAt   time.Time `json:"createdAt"`
}

// VolumeSchedule represents an automated backup cron policy.
type VolumeSchedule struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"projectId"`
	VolumeName     string     `json:"volumeName"`
	Cron           string     `json:"cron"`
	Label          string     `json:"label"`
	RetentionCount int        `json:"retentionCount"`
	Enabled        bool       `json:"enabled"`
	LastRun        *time.Time `json:"lastRun,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// Deployment represents a build/deploy execution record.
type Deployment struct {
	ID            string     `json:"id"`
	ProjectID     string     `json:"projectId"`
	ServiceID     string     `json:"serviceId"`
	Version       string     `json:"version"`
	CommitHash    string     `json:"commitHash"`
	CommitMessage string     `json:"commitMessage"`
	Status        string     `json:"status"`
	Duration      string     `json:"duration"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
}
