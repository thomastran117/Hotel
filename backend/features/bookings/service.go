package bookings

import (
	"context"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// Service implements business logic for the bookings feature.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create validates dates, resolves the caller's customer profile, and creates a booking.
func (s *Service) Create(ctx context.Context, userID, roomID uint, checkIn, checkOut string) (*Booking, error) {
	if err := validateDates(checkIn, checkOut); err != nil {
		return nil, err
	}

	customerID, err := s.repo.FindCustomerIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, BookingInput{
		CustomerID:   customerID,
		RoomID:       roomID,
		CheckInDate:  checkIn,
		CheckOutDate: checkOut,
	})
}

// Get returns a booking by ID. Guests may only view their own bookings.
func (s *Service) Get(ctx context.Context, id, userID uint, role string) (*Booking, error) {
	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == "guest" {
		customerID, cerr := s.repo.FindCustomerIDByUserID(ctx, userID)
		if cerr != nil || b.CustomerID == nil || *b.CustomerID != customerID {
			return nil, ErrNotOwner
		}
	}
	return b, nil
}

// List returns bookings. Guests see only their own; staff/admin see all.
func (s *Service) List(ctx context.Context, userID uint, role, status string, limit, offset int) ([]*Booking, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	f := BookingFilter{Limit: limit, Offset: offset}
	if status != "" {
		f.Status = &status
	}

	if role == "guest" {
		customerID, err := s.repo.FindCustomerIDByUserID(ctx, userID)
		if err != nil {
			// No customer profile — return empty list rather than an error.
			return []*Booking{}, nil
		}
		f.CustomerID = &customerID
	}

	return s.repo.List(ctx, f)
}

// Cancel cancels a booking. Guests may only cancel their own bookings.
func (s *Service) Cancel(ctx context.Context, id, userID uint, role string) error {
	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if role == "guest" {
		customerID, cerr := s.repo.FindCustomerIDByUserID(ctx, userID)
		if cerr != nil || b.CustomerID == nil || *b.CustomerID != customerID {
			return ErrNotOwner
		}
	}
	return s.repo.Cancel(ctx, id)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func validateDates(checkIn, checkOut string) error {
	in, err := time.Parse(dateLayout, checkIn)
	if err != nil {
		return fmt.Errorf("%w: invalid check-in date", ErrInvalidDates)
	}
	out, err := time.Parse(dateLayout, checkOut)
	if err != nil {
		return fmt.Errorf("%w: invalid check-out date", ErrInvalidDates)
	}
	if !out.After(in) {
		return ErrInvalidDates
	}
	return nil
}
