package projects

import "time"

// Project represents a workspace grouping of services and domains.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

// CreateProjectInput holds data for creating a project.
type CreateProjectInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
