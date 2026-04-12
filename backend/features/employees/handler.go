package employees

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the employees feature.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Request types ─────────────────────────────────────────────────────────────

type positionRequest struct {
	HotelID  uint   `json:"hotel_id"  binding:"required"`
	Position string `json:"position"  binding:"required,max=100"`
}

type employeeRequest struct {
	FirstName string            `json:"first_name" binding:"required,max=100"`
	LastName  string            `json:"last_name"  binding:"required,max=100"`
	Street    string            `json:"street"     binding:"required,max=255"`
	City      string            `json:"city"       binding:"required,max=100"`
	State     string            `json:"state"      binding:"required,max=100"`
	Country   string            `json:"country"    binding:"required,max=100"`
	Zip       string            `json:"zip"        binding:"required,max=20"`
	SSN       string            `json:"ssn"        binding:"required,max=50"`
	Positions []positionRequest `json:"positions"`
}

type listQuery struct {
	HotelID *uint   `form:"hotel_id"`
	Name    *string `form:"name"`
	Limit   int     `form:"limit"`
	Offset  int     `form:"offset"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetMe godoc
// GET /employees/me
func (h *Handler) GetMe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "unauthorized", nil))
		return
	}

	emp, err := h.service.GetMe(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrEmployeeNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "no employee record linked to this account", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, emp)
}

// UpdateMyPositions godoc
// PUT /employees/me/positions
func (h *Handler) UpdateMyPositions(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "unauthorized", nil))
		return
	}

	var req []positionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	positions := make([]PositionInput, len(req))
	for i, p := range req {
		positions[i] = PositionInput{HotelID: p.HotelID, Position: p.Position}
	}

	emp, err := h.service.UpdateMyPositions(c.Request.Context(), userID, positions)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmployeeNotFound):
			_ = c.Error(middleware.New(http.StatusNotFound, "no employee record linked to this account", err))
		case errors.Is(err, ErrHotelNotFound):
			_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "hotel not found", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	c.JSON(http.StatusOK, emp)
}

// GetAll godoc
// GET /employees
func (h *Handler) GetAll(c *gin.Context) {
	var q listQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	filter := EmployeeFilter{
		HotelID: q.HotelID,
		Name:    q.Name,
		Limit:   q.Limit,
		Offset:  q.Offset,
	}

	employees, err := h.service.GetAll(c.Request.Context(), filter)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, employees)
}

// GetByID godoc
// GET /employees/:id
func (h *Handler) GetByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	employee, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrEmployeeNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "employee not found", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, employee)
}

// Update godoc
// PUT /employees/:id
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req employeeRequest
	if !bindJSON(c, &req) {
		return
	}

	employee, err := h.service.Update(c.Request.Context(), id, toInput(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrEmployeeNotFound):
			_ = c.Error(middleware.New(http.StatusNotFound, "employee not found", err))
		case errors.Is(err, ErrSSNTaken):
			_ = c.Error(middleware.New(http.StatusConflict, "SSN already in use", err))
		case errors.Is(err, ErrHotelNotFound):
			_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "hotel not found", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	c.JSON(http.StatusOK, employee)
}

// Delete godoc
// DELETE /employees/:id
func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrEmployeeNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "employee not found", err))
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

func toInput(req employeeRequest) EmployeeInput {
	positions := make([]PositionInput, len(req.Positions))
	for i, p := range req.Positions {
		positions[i] = PositionInput{HotelID: p.HotelID, Position: p.Position}
	}
	return EmployeeInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Street:    req.Street,
		City:      req.City,
		State:     req.State,
		Country:   req.Country,
		Zip:       req.Zip,
		SSN:       req.SSN,
		Positions: positions,
	}
}
