package projects

import (
	"time"

	"gopod/internal/services"
)

// ProjectDomain represents a domain attached to a service in the project.
type ProjectDomain struct {
	ID        string `json:"id"`
	Hostname  string `json:"hostname"`
	ServiceID string `json:"serviceId"`
	TLS       bool   `json:"tls"`
	Status    string `json:"status"`
}

// Project represents a workspace grouping of services and domains with backend-computed health and metrics.
type Project struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Status       string             `json:"status"`
	Services     []services.Service `json:"services"`
	ServiceCount int                `json:"serviceCount"`
	Domains      []ProjectDomain    `json:"domains"`
	DomainCount  int                `json:"domainCount"`
	CPU          float64            `json:"cpu"`
	Memory       int64              `json:"memory"`
	MemoryTotal  int64              `json:"memoryTotal"`
	CreatedAt    time.Time          `json:"createdAt"`
}

// CreateProjectInput holds data for creating a project.
type CreateProjectInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
