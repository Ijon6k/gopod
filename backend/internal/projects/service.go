package projects

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Service encapsulates project management business logic.
type Service struct {
	repo Repository
}

// NewService creates a new projects domain service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListProjects retrieves all projects.
func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	if s.repo == nil {
		return []Project{}, nil
	}
	return s.repo.List(ctx)
}

// GetProject retrieves a single project by ID.
func (s *Service) GetProject(ctx context.Context, id string) (*Project, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	return s.repo.Get(ctx, id)
}

// CreateProject validates input, generates slug/ID, and persists a new project.
func (s *Service) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("project name is required")
	}

	id := strings.ToLower(strings.TrimSpace(input.ID))
	if id == "" {
		id = strings.ReplaceAll(strings.ToLower(name), " ", "-")
		cleanID := ""
		for _, r := range id {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				cleanID += string(r)
			}
		}
		id = cleanID
	}
	if id == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		id = fmt.Sprintf("proj-%s", hex.EncodeToString(b))
	}

	// Check if already exists
	existing, err := s.repo.Get(ctx, id)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("project with ID '%s' already exists", id)
	}

	project := Project{
		ID:          id,
		Name:        name,
		Description: input.Description,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, project); err != nil {
		return nil, err
	}

	return &project, nil
}

// DeleteProject deletes a project by ID.
func (s *Service) DeleteProject(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	return s.repo.Delete(ctx, id)
}
