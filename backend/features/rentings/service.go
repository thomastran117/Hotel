package rentings

import (
	"context"
	"fmt"
	"time"
)

// Service implements business logic for the rentings feature.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateWalkIn resolves the employee profile and creates a walk-in renting.
func (s *Service) CreateWalkIn(ctx context.Context, userID, customerID, roomID uint, checkInDate string) (*Renting, error) {
	if err := validateDate(checkInDate); err != nil {
		return nil, err
	}

	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateWalkIn(ctx, WalkInInput{
		CustomerID:  customerID,
		RoomID:      roomID,
		EmployeeID:  employeeID,
		CheckInDate: checkInDate,
	})
}

// CreateFromBooking resolves the employee profile and converts a booking to a renting.
func (s *Service) CreateFromBooking(ctx context.Context, bookingID, userID uint) (*Renting, error) {
	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateFromBooking(ctx, bookingID, employeeID)
}

// Get returns a renting by ID. Guests may only view their own rentings.
func (s *Service) Get(ctx context.Context, id, userID uint, role string) (*Renting, error) {
	rt, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == "guest" {
		customerID, cerr := s.repo.FindCustomerIDByUserID(ctx, userID)
		if cerr != nil || rt.CustomerID == nil || *rt.CustomerID != customerID {
			return nil, ErrNotOwner
		}
	}
	return rt, nil
}

// List returns rentings. Guests see only their own; staff/admin see all.
func (s *Service) List(ctx context.Context, userID uint, role, status string, limit, offset int) ([]*Renting, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	f := RentingFilter{Limit: limit, Offset: offset}
	if status != "" {
		f.Status = &status
	}

	if role == "guest" {
		customerID, err := s.repo.FindCustomerIDByUserID(ctx, userID)
		if err != nil {
			// No customer profile — return empty list rather than an error.
			return []*Renting{}, nil
		}
		f.CustomerID = &customerID
	}

	return s.repo.List(ctx, f)
}

// CheckOut completes a renting and records the payment.
func (s *Service) CheckOut(ctx context.Context, id uint, checkOutDate, paymentMethod string) (*Renting, error) {
	return s.repo.CheckOut(ctx, id, checkOutDate, paymentMethod)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func validateDate(date string) error {
	_, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("%w: invalid date format, expected YYYY-MM-DD", ErrInvalidCheckout)
	}
	return nil
}
