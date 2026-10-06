package projects

import "context"

// Repository defines data access contract for projects.
type Repository interface {
	List(ctx context.Context) ([]Project, error)
	Get(ctx context.Context, id string) (*Project, error)
	Create(ctx context.Context, p Project) error
	Delete(ctx context.Context, id string) error
}
