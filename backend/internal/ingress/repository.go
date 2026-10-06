package ingress

import "context"

// Repository defines data access contract for domains.
type Repository interface {
	List(ctx context.Context, projectID string) ([]Domain, error)
	Get(ctx context.Context, id string) (*Domain, error)
	Create(ctx context.Context, d Domain) error
	Update(ctx context.Context, d Domain) error
	Delete(ctx context.Context, id string) error
}
