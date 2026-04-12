package customers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the customers feature.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Request types ─────────────────────────────────────────────────────────────

type customerRequest struct {
	FirstName string `json:"first_name" binding:"required,max=100"`
	LastName  string `json:"last_name"  binding:"required,max=100"`
	Street    string `json:"street"     binding:"required,max=255"`
	City      string `json:"city"       binding:"required,max=100"`
	State     string `json:"state"      binding:"required,max=100"`
	Country   string `json:"country"    binding:"required,max=100"`
	Zip       string `json:"zip"        binding:"required,max=20"`
	IDType    string `json:"id_type"    binding:"required,oneof=SSN SIN driving_licence"`
	IDNumber  string `json:"id_number"  binding:"required,max=100"`
}

type listQuery struct {
	Name   *string `form:"name"`
	Limit  int     `form:"limit"`
	Offset int     `form:"offset"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetAll godoc
// GET /customers
func (h *Handler) GetAll(c *gin.Context) {
	var q listQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	filter := CustomerFilter{
		Name:   q.Name,
		Limit:  q.Limit,
		Offset: q.Offset,
	}

	customers, err := h.service.GetAll(c.Request.Context(), filter)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, customers)
}

// GetByID godoc
// GET /customers/:id
func (h *Handler) GetByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	customer, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrCustomerNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "customer not found", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, customer)
}

// Update godoc
// PUT /customers/:id
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req customerRequest
	if !bindJSON(c, &req) {
		return
	}

	customer, err := h.service.Update(c.Request.Context(), id, toInput(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrCustomerNotFound):
			_ = c.Error(middleware.New(http.StatusNotFound, "customer not found", err))
		case errors.Is(err, ErrIdentityTaken):
			_ = c.Error(middleware.New(http.StatusConflict, "identity document already in use", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	c.JSON(http.StatusOK, customer)
}

// Delete godoc
// DELETE /customers/:id
func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrCustomerNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "customer not found", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.Status(http.StatusNoContent)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func bindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return false
	}
	return true
}

func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, "invalid id", err))
		c.Abort()
		return 0, false
	}
	return uint(n), true
}

func toInput(req customerRequest) CustomerInput {
	return CustomerInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Street:    req.Street,
		City:      req.City,
		State:     req.State,
		Country:   req.Country,
		Zip:       req.Zip,
		IDType:    req.IDType,
		IDNumber:  req.IDNumber,
	}
}
