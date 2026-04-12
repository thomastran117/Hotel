package bookings

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the bookings feature.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Request types ─────────────────────────────────────────────────────────────

type createRequest struct {
	RoomID       uint   `json:"room_id"       binding:"required"`
	CheckInDate  string `json:"check_in_date"  binding:"required"`
	CheckOutDate string `json:"check_out_date" binding:"required"`
}

type listQuery struct {
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Create godoc
// POST /bookings
func (h *Handler) Create(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "authentication required", nil))
		c.Abort()
		return
	}

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	booking, err := h.service.Create(c.Request.Context(), userID, req.RoomID, req.CheckInDate, req.CheckOutDate)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, booking)
}

// List godoc
// GET /bookings
func (h *Handler) List(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "authentication required", nil))
		c.Abort()
		return
	}

	role, _ := middleware.GetRole(c)

	var q listQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	bookings, err := h.service.List(c.Request.Context(), userID, role, q.Status, q.Limit, q.Offset)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, bookings)
}

// Get godoc
// GET /bookings/:id
func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "authentication required", nil))
		c.Abort()
		return
	}

	role, _ := middleware.GetRole(c)

	booking, err := h.service.Get(c.Request.Context(), id, userID, role)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

// Cancel godoc
// DELETE /bookings/:id
func (h *Handler) Cancel(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "authentication required", nil))
		c.Abort()
		return
	}

	role, _ := middleware.GetRole(c)

	if err := h.service.Cancel(c.Request.Context(), id, userID, role); err != nil {
		h.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrBookingNotFound):
		_ = c.Error(middleware.New(http.StatusNotFound, "booking not found", err))
	case errors.Is(err, ErrRoomNotFound):
		_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "room not found", err))
	case errors.Is(err, ErrCustomerNotFound):
		_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "customer profile not found — please complete your profile before booking", err))
	case errors.Is(err, ErrRoomUnavailable):
		_ = c.Error(middleware.New(http.StatusConflict, "room is not available for the requested dates", err))
	case errors.Is(err, ErrInvalidDates):
		_ = c.Error(middleware.New(http.StatusBadRequest, "check-out date must be after check-in date", err))
	case errors.Is(err, ErrNotOwner):
		_ = c.Error(middleware.New(http.StatusForbidden, "you do not have access to this booking", err))
	case errors.Is(err, ErrAlreadyCancelled):
		_ = c.Error(middleware.New(http.StatusConflict, "booking is already cancelled", err))
	case errors.Is(err, ErrAlreadyCheckedIn):
		_ = c.Error(middleware.New(http.StatusConflict, "booking has already been checked in", err))
	default:
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
	}
}

func parseID(c *gin.Context, param string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, "invalid id", err))
		c.Abort()
		return 0, false
	}
	return uint(n), true
}
