package projects

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopod/internal/services"
)

// ServiceChecker checks the number of services running inside a project.
type ServiceChecker func(ctx context.Context, projectID string) (int, error)

// ServiceProvider returns all services for a specific project.
type ServiceProvider func(ctx context.Context, projectID string) ([]services.Service, error)

// DomainProvider returns all domains for a specific project.
type DomainProvider func(ctx context.Context, projectID string) ([]ProjectDomain, error)

// Service encapsulates project management business logic.
type Service struct {
	repo            Repository
	serviceChecker  ServiceChecker
	serviceProvider ServiceProvider
	domainProvider  DomainProvider
}

// NewService creates a new projects domain service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SetServiceChecker configures the active services checker.
func (s *Service) SetServiceChecker(checker ServiceChecker) {
	s.serviceChecker = checker
}

// SetProviders configures service and domain providers for rich project aggregation.
func (s *Service) SetProviders(sp ServiceProvider, dp DomainProvider) {
	s.serviceProvider = sp
	s.domainProvider = dp
}

// enrichProject populates child services, domains, metrics, and health status on the project.
func (s *Service) enrichProject(ctx context.Context, p *Project) {
	if p.Services == nil {
		p.Services = []services.Service{}
	}
	if p.Domains == nil {
		p.Domains = []ProjectDomain{}
	}
	p.MemoryTotal = 16384 // 16GB baseline

	if s.serviceProvider != nil {
		svcs, err := s.serviceProvider(ctx, p.ID)
		if err == nil && svcs != nil {
			p.Services = svcs
		}
	}
	p.ServiceCount = len(p.Services)

	if s.domainProvider != nil {
		doms, err := s.domainProvider(ctx, p.ID)
		if err == nil && doms != nil {
			p.Domains = doms
		}
	}
	p.DomainCount = len(p.Domains)

	// Calculate aggregated metrics & health status across services
	var totalCPU float64
	var totalMem int64
	hasDeploying := false
	hasFailed := false
	allStopped := len(p.Services) > 0

	for _, svc := range p.Services {
		totalCPU += svc.CPULimit
		totalMem += int64(svc.MemoryLimit)

		switch svc.Status {
		case "deploying", "building", "starting":
			hasDeploying = true
			allStopped = false
		case "failed", "unhealthy", "degraded":
			hasFailed = true
			allStopped = false
		case "running", "healthy":
			allStopped = false
		}
	}

	p.CPU = totalCPU
	p.Memory = totalMem

	if len(p.Services) == 0 {
		p.Status = "healthy"
	} else if hasDeploying {
		p.Status = "deploying"
	} else if hasFailed {
		p.Status = "degraded"
	} else if allStopped {
		p.Status = "stopped"
	} else {
		p.Status = "healthy"
	}
}

// ListProjects retrieves all projects with rich pre-computed telemetry and child items.
func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	if s.repo == nil {
		return []Project{}, nil
	}
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		s.enrichProject(ctx, &list[i])
	}
	return list, nil
}

// GetProject retrieves a single project by ID with rich child items.
func (s *Service) GetProject(ctx context.Context, id string) (*Project, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p != nil {
		s.enrichProject(ctx, p)
	}
	return p, nil
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
		Status:      "healthy",
		Services:    []services.Service{},
		Domains:     []ProjectDomain{},
		CreatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, project); err != nil {
		return nil, err
	}

	s.enrichProject(ctx, &project)
	return &project, nil
}

// DeleteProject deletes a project by ID, preventing deletion if services exist.
func (s *Service) DeleteProject(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	if s.serviceChecker != nil {
		count, err := s.serviceChecker(ctx, id)
		if err == nil && count > 0 {
			return fmt.Errorf("cannot delete project: project contains %d active service(s), please delete them first", count)
		}
	}
	return s.repo.Delete(ctx, id)
}
