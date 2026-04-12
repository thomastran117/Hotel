package employees

import "context"

// Service implements business logic for the employees feature.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetByID returns an employee by ID.
func (s *Service) GetByID(ctx context.Context, id uint) (*Employee, error) {
	return s.repo.FindByID(ctx, id)
}

// GetMe returns the employee record linked to the given user ID.
func (s *Service) GetMe(ctx context.Context, userID uint) (*Employee, error) {
	return s.repo.FindByUserID(ctx, userID)
}

// UpdateMyPositions replaces positions for the employee linked to the given user ID.
func (s *Service) UpdateMyPositions(ctx context.Context, userID uint, positions []PositionInput) (*Employee, error) {
	emp, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ReplacePositions(ctx, emp.ID, positions)
}

// GetAll returns employees matching the given filter.
func (s *Service) GetAll(ctx context.Context, f EmployeeFilter) ([]*Employee, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.FindAll(ctx, f)
}

// Update replaces employee fields and positions.
func (s *Service) Update(ctx context.Context, id uint, in EmployeeInput) (*Employee, error) {
	return s.repo.Update(ctx, id, in)
}

// Delete removes an employee by ID.
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}
