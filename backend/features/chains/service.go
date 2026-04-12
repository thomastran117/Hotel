package chains

import (
	"context"
	"fmt"
)

// Service implements business logic for the chains feature.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create adds a new hotel chain.
func (s *Service) Create(ctx context.Context, in ChainInput) (*Chain, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, in)
}

// GetByID returns a chain by ID.
func (s *Service) GetByID(ctx context.Context, id uint) (*Chain, error) {
	return s.repo.FindByID(ctx, id)
}

// List returns all chains.
func (s *Service) List(ctx context.Context) ([]*Chain, error) {
	return s.repo.List(ctx)
}

// Update replaces chain fields and contacts.
func (s *Service) Update(ctx context.Context, id uint, in ChainInput) (*Chain, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, in)
}

// Delete removes a chain by ID.
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// ErrInvalidContact is returned when a contact has an unrecognised type.
var ErrInvalidContact = fmt.Errorf("chains: contact type must be 'email' or 'phone'")

func validateInput(in ChainInput) error {
	for _, c := range in.Contacts {
		if c.Type != "email" && c.Type != "phone" {
			return ErrInvalidContact
		}
	}
	return nil
}
