package reviews

import "context"

// Service implements business logic for the reviews feature.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create adds a new review for a hotel by the given user.
func (s *Service) Create(ctx context.Context, in ReviewInput) (*Review, error) {
	return s.repo.Create(ctx, in)
}

// ListByHotel returns paginated reviews for a hotel.
func (s *Service) ListByHotel(ctx context.Context, hotelID uint, limit, offset int) ([]*Review, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListByHotel(ctx, hotelID, limit, offset)
}

// Delete removes a review. Only the review's author or an admin may delete it.
func (s *Service) Delete(ctx context.Context, reviewID, userID uint, role string) error {
	rev, err := s.repo.FindByID(ctx, reviewID)
	if err != nil {
		return err
	}
	if role != "admin" && (rev.UserID == nil || *rev.UserID != userID) {
		return ErrNotOwner
	}
	return s.repo.Delete(ctx, reviewID)
}
