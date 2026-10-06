package storage

import "time"

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
