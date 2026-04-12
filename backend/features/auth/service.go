package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"hotel/application/cache"
)

// TokenPair holds the access and refresh tokens returned after auth operations.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// Service implements auth business logic.
type Service struct {
	repo      *Repository
	cache     cache.Store
	jwtSecret string
}

// NewService creates a Service with the given dependencies.
func NewService(repo *Repository, cache cache.Store, jwtSecret string) *Service {
	return &Service{repo: repo, cache: cache, jwtSecret: jwtSecret}
}

// SignupInput carries all fields required to create a user and their profile.
// Role must be "guest", "staff", or "admin".
type SignupInput struct {
	Email     string
	Password  string
	Role      string
	FirstName string
	LastName  string
	Street    string
	City      string
	State     string
	Country   string
	Zip       string
	// guest only
	IDType   string
	IDNumber string
	// staff / admin only
	SSN string
}

// Signup creates a new account with the appropriate profile and returns a token pair.
// For guests a customers record is created; for staff/admin an employees record is created.
func (s *Service) Signup(ctx context.Context, in SignupInput) (TokenPair, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth: hash password: %w", err)
	}

	var user *User

	switch in.Role {
	case "guest":
		user, err = s.repo.CreateWithCustomer(ctx, in.Email, string(hash), CustomerProfile{
			FirstName: in.FirstName,
			LastName:  in.LastName,
			Street:    in.Street,
			City:      in.City,
			State:     in.State,
			Country:   in.Country,
			Zip:       in.Zip,
			IDType:    in.IDType,
			IDNumber:  in.IDNumber,
		})
	case "staff", "admin":
		user, err = s.repo.CreateWithEmployee(ctx, in.Email, string(hash), in.Role, EmployeeProfile{
			FirstName: in.FirstName,
			LastName:  in.LastName,
			Street:    in.Street,
			City:      in.City,
			State:     in.State,
			Country:   in.Country,
			Zip:       in.Zip,
			SSN:       in.SSN,
		})
	default:
		return TokenPair{}, fmt.Errorf("auth: unknown role %q", in.Role)
	}

	if err != nil {
		return TokenPair{}, err
	}

	return s.issueTokenPair(ctx, user.ID, user.Role)
}

// Login authenticates a user by email/password and returns a token pair.
func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		// Mask not-found as invalid credentials to prevent user enumeration.
		if errors.Is(err, ErrUserNotFound) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	return s.issueTokenPair(ctx, user.ID, user.Role)
}

// Refresh validates a refresh token, rotates it, and returns a new token pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	key := refreshTokenKey(refreshToken)

	val, err := s.cache.Get(ctx, key)
	if err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			return TokenPair{}, ErrInvalidRefreshToken
		}
		return TokenPair{}, fmt.Errorf("auth: refresh lookup: %w", err)
	}

	userID, role, err := parseTokenPayload(val)
	if err != nil {
		return TokenPair{}, ErrInvalidRefreshToken
	}

	// Rotate: delete the old token before issuing a new one.
	if err := s.cache.Delete(ctx, key); err != nil {
		return TokenPair{}, fmt.Errorf("auth: delete old refresh token: %w", err)
	}

	return s.issueTokenPair(ctx, userID, role)
}

// Logout invalidates the refresh token.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	key := refreshTokenKey(refreshToken)
	if err := s.cache.Delete(ctx, key); err != nil {
		return fmt.Errorf("auth: logout: %w", err)
	}
	return nil
}

// issueTokenPair generates and stores a new access + refresh token pair.
func (s *Service) issueTokenPair(ctx context.Context, userID uint, role string) (TokenPair, error) {
	access, err := generateAccessToken(s.jwtSecret, userID, role)
	if err != nil {
		return TokenPair{}, err
	}

	refresh := newRefreshToken()
	payload := buildTokenPayload(userID, role)

	if err := s.cache.Set(ctx, refreshTokenKey(refresh), payload, refreshTokenTTL); err != nil {
		return TokenPair{}, fmt.Errorf("auth: store refresh token: %w", err)
	}

	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// buildTokenPayload encodes userID and role as "id:role" for Redis storage.
func buildTokenPayload(userID uint, role string) string {
	return strconv.FormatUint(uint64(userID), 10) + ":" + role
}

// parseTokenPayload decodes the "id:role" string stored in Redis.
func parseTokenPayload(val string) (uint, string, error) {
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("auth: malformed token payload")
	}
	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("auth: invalid user id in token payload")
	}
	return uint(id), parts[1], nil
}

// Sentinel errors returned by the service.
var (
	ErrInvalidCredentials  = errors.New("auth: invalid email or password")
	ErrInvalidRefreshToken = errors.New("auth: invalid or expired refresh token")
)
