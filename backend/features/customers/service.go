package customers

import "context"

// Service implements business logic for the customers feature.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetByID returns a customer by ID.
func (s *Service) GetByID(ctx context.Context, id uint) (*Customer, error) {
	return s.repo.FindByID(ctx, id)
}

// GetAll returns customers matching the given filter.
func (s *Service) GetAll(ctx context.Context, f CustomerFilter) ([]*Customer, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.FindAll(ctx, f)
}

// Update replaces customer fields.
func (s *Service) Update(ctx context.Context, id uint, in CustomerInput) (*Customer, error) {
	return s.repo.Update(ctx, id, in)
}

// Delete removes a customer by ID.
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}
