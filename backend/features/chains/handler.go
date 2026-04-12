package chains

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the chains feature.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Request / response types ──────────────────────────────────────────────────

type contactRequest struct {
	Type  string `json:"type"  binding:"required,oneof=email phone"`
	Value string `json:"value" binding:"required,max=255"`
}

type chainRequest struct {
	Name          string           `json:"name"           binding:"required,max=255"`
	OfficeStreet  string           `json:"office_street"  binding:"required,max=255"`
	OfficeCity    string           `json:"office_city"    binding:"required,max=100"`
	OfficeState   string           `json:"office_state"   binding:"required,max=100"`
	OfficeCountry string           `json:"office_country" binding:"required,max=100"`
	OfficeZip     string           `json:"office_zip"     binding:"required,max=20"`
	Contacts      []contactRequest `json:"contacts"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Create godoc
// POST /chains
func (h *Handler) Create(c *gin.Context) {
	var req chainRequest
	if !bindJSON(c, &req) {
		return
	}

	chain, err := h.service.Create(c.Request.Context(), toInput(req))
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusCreated, chain)
}

// List godoc
// GET /chains
func (h *Handler) List(c *gin.Context) {
	chains, err := h.service.List(c.Request.Context())
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}
	c.JSON(http.StatusOK, chains)
}

// GetByID godoc
// GET /chains/:id
func (h *Handler) GetByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	chain, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrChainNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "chain not found", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, chain)
}

// Update godoc
// PUT /chains/:id
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req chainRequest
	if !bindJSON(c, &req) {
		return
	}

	chain, err := h.service.Update(c.Request.Context(), id, toInput(req))
	if err != nil {
		if errors.Is(err, ErrChainNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "chain not found", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, chain)
}

// Delete godoc
// DELETE /chains/:id
func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrChainNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "chain not found", err))
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

func toInput(req chainRequest) ChainInput {
	contacts := make([]ContactInput, len(req.Contacts))
	for i, ct := range req.Contacts {
		contacts[i] = ContactInput{Type: ct.Type, Value: ct.Value}
	}
	return ChainInput{
		Name:          req.Name,
		OfficeStreet:  req.OfficeStreet,
		OfficeCity:    req.OfficeCity,
		OfficeState:   req.OfficeState,
		OfficeCountry: req.OfficeCountry,
		OfficeZip:     req.OfficeZip,
		Contacts:      contacts,
	}
}
