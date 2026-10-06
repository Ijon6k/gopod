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
	CreateDeployment(ctx context.Context, d Deployment) error
	ListDeployments(ctx context.Context, serviceID string) ([]Deployment, error)
}
