package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the auth feature.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Request / response types ──────────────────────────────────────────────────

type signupRequest struct {
	// auth
	Email    string `json:"email"    binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Role     string `json:"role"     binding:"required,oneof=guest staff admin"`
	// profile — common to all roles
	FirstName string `json:"first_name" binding:"required,max=100"`
	LastName  string `json:"last_name"  binding:"required,max=100"`
	Street    string `json:"street"     binding:"required,max=255"`
	City      string `json:"city"       binding:"required,max=100"`
	State     string `json:"state"      binding:"required,max=100"`
	Country   string `json:"country"    binding:"required,max=100"`
	Zip       string `json:"zip"        binding:"required,max=20"`
	// guest only
	IDType   string `json:"id_type"   binding:"omitempty,oneof=SSN SIN driving_licence"`
	IDNumber string `json:"id_number" binding:"omitempty,max=100"`
	// staff / admin only
	SSN string `json:"ssn" binding:"omitempty,max=50"`
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

// ── Cookie helpers ────────────────────────────────────────────────────────────

const (
	refreshCookieName   = "refresh_token"
	refreshCookiePath   = "/auth"
	refreshCookieMaxAge = 7 * 24 * 60 * 60 // 7 days in seconds
)

func setRefreshCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, token, refreshCookieMaxAge, refreshCookiePath, "", false, true)
}

func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, "", false, true)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Signup godoc
// POST /auth/signup
func (h *Handler) Signup(c *gin.Context) {
	var req signupRequest
	if !bindJSON(c, &req) {
		return
	}

	// Validate role-specific required fields after binding.
	switch req.Role {
	case "guest":
		if req.IDType == "" || req.IDNumber == "" {
			_ = c.Error(middleware.New(http.StatusBadRequest, "id_type and id_number are required for guest accounts", nil))
			return
		}
	case "staff", "admin":
		if req.SSN == "" {
			_ = c.Error(middleware.New(http.StatusBadRequest, "ssn is required for staff and admin accounts", nil))
			return
		}
	}

	pair, err := h.service.Signup(c.Request.Context(), SignupInput{
		Email:     req.Email,
		Password:  req.Password,
		Role:      req.Role,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Street:    req.Street,
		City:      req.City,
		State:     req.State,
		Country:   req.Country,
		Zip:       req.Zip,
		IDType:    req.IDType,
		IDNumber:  req.IDNumber,
		SSN:       req.SSN,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			_ = c.Error(middleware.New(http.StatusConflict, "email already in use", err))
		case errors.Is(err, ErrIdentityTaken):
			_ = c.Error(middleware.New(http.StatusConflict, "identity document already registered", err))
		case errors.Is(err, ErrSSNTaken):
			_ = c.Error(middleware.New(http.StatusConflict, "SSN already registered", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	setRefreshCookie(c, pair.RefreshToken)
	c.JSON(http.StatusCreated, tokenResponse{AccessToken: pair.AccessToken})
}

// Login godoc
// POST /auth/login
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req) {
		return
	}

	pair, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			_ = c.Error(middleware.New(http.StatusUnauthorized, "invalid email or password", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	setRefreshCookie(c, pair.RefreshToken)
	c.JSON(http.StatusOK, tokenResponse{AccessToken: pair.AccessToken})
}

// Refresh godoc
// POST /auth/refresh
func (h *Handler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "missing refresh token", err))
		return
	}

	pair, err := h.service.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			clearRefreshCookie(c)
			_ = c.Error(middleware.New(http.StatusUnauthorized, "invalid or expired refresh token", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	setRefreshCookie(c, pair.RefreshToken)
	c.JSON(http.StatusOK, tokenResponse{AccessToken: pair.AccessToken})
}

// Logout godoc
// POST /auth/logout
func (h *Handler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil {
		// Nothing to invalidate — treat as already logged out.
		c.Status(http.StatusNoContent)
		return
	}

	if err := h.service.Logout(c.Request.Context(), refreshToken); err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// bindJSON deserialises and validates the request body. On failure it attaches
// a 400 AppError to the context and aborts so ErrorHandler can respond.
func bindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return false
	}
	return true
}
