package hotels

import (
	"context"
	"fmt"
)

// Service implements business logic for the hotels feature.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create adds a new hotel.
func (s *Service) Create(ctx context.Context, in HotelInput) (*Hotel, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, in)
}

// GetByID returns a hotel by ID.
func (s *Service) GetByID(ctx context.Context, id uint) (*Hotel, error) {
	return s.repo.FindByID(ctx, id)
}

// Search returns hotels matching the given filter.
func (s *Service) Search(ctx context.Context, f HotelFilter) ([]*Hotel, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.Search(ctx, f)
}

// Update replaces hotel fields and contacts.
func (s *Service) Update(ctx context.Context, id uint, in HotelInput) (*Hotel, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, in)
}

// Delete removes a hotel by ID.
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// ErrInvalidContact is returned when a contact has an unrecognised type.
var ErrInvalidContact = fmt.Errorf("hotels: contact type must be 'email' or 'phone'")

func validateInput(in HotelInput) error {
	for _, c := range in.Contacts {
		if c.Type != "email" && c.Type != "phone" {
			return ErrInvalidContact
		}
	}
	return nil
}
