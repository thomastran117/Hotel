package rooms

import (
	"context"
	"time"
)

// Service implements business logic for the rooms feature.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create adds a new room with its amenities and damages.
func (s *Service) Create(ctx context.Context, in RoomInput) (*Room, error) {
	return s.repo.Create(ctx, in)
}

// GetByID returns a room by ID.
func (s *Service) GetByID(ctx context.Context, id uint) (*Room, error) {
	return s.repo.FindByID(ctx, id)
}

// Search returns rooms matching the given filter.
// CheckIn defaults to today and CheckOut defaults to today+7 when not provided,
// so availability is always evaluated against a concrete window.
func (s *Service) Search(ctx context.Context, f RoomFilter) ([]*Room, error) {
	today := time.Now().Format("2006-01-02")
	weekLater := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	if f.CheckIn == nil {
		f.CheckIn = &today
	}
	if f.CheckOut == nil {
		f.CheckOut = &weekLater
	}
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.Search(ctx, f)
}

// Update replaces room fields, amenities, and damages.
func (s *Service) Update(ctx context.Context, id uint, in RoomInput) (*Room, error) {
	return s.repo.Update(ctx, id, in)
}

// Delete removes a room by ID.
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}
