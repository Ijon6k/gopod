package services

import "context"

// Repository defines data access contract for services and deployments.
type Repository interface {
	List(ctx context.Context, projectID string) ([]Service, error)
	Get(ctx context.Context, id string) (*Service, error)
	Create(ctx context.Context, s Service) error
	Update(ctx context.Context, s Service) error
	Delete(ctx context.Context, id string) error
	GetByWebhookToken(ctx context.Context, token string) (*Service, error)
	SetWebhookToken(ctx context.Context, id, token string) error
	SetWebhookSecret(ctx context.Context, id, secret string) error
	CreateDeployment(ctx context.Context, d Deployment) error
	UpdateDeployment(ctx context.Context, d Deployment) error
	GetDeployment(ctx context.Context, id string) (*Deployment, error)
	ListDeployments(ctx context.Context, serviceID string) ([]Deployment, error)
	// NextDeploymentNumber returns max(number)+1 across the service's full
	// deployment history (ListDeployments is a UI window capped at 50 rows and
	// must never drive sequence logic).
	NextDeploymentNumber(ctx context.Context, serviceID string) (int, error)
	// ListRunningDeployments selects non-terminal deployments of a service,
	// unbounded: cancel/reset sweeps must never skip an in-flight row that
	// aged out of a LIMIT window.
	ListRunningDeployments(ctx context.Context, serviceID string) ([]Deployment, error)
}
