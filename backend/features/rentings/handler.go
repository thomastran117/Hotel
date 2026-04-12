package rentings

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the rentings feature.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Request types ─────────────────────────────────────────────────────────────

type walkInRequest struct {
	CustomerID  uint   `json:"customer_id"  binding:"required"`
	RoomID      uint   `json:"room_id"      binding:"required"`
	CheckInDate string `json:"check_in_date" binding:"required"`
}

type checkOutRequest struct {
	CheckOutDate  string `json:"check_out_date"  binding:"required"`
	PaymentMethod string `json:"payment_method"  binding:"required"`
}

type listQuery struct {
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// CreateWalkIn godoc
// POST /rentings
func (h *Handler) CreateWalkIn(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "authentication required", nil))
		c.Abort()
		return
	}

	var req walkInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	renting, err := h.service.CreateWalkIn(c.Request.Context(), userID, req.CustomerID, req.RoomID, req.CheckInDate)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, renting)
}

// CheckIn godoc
// POST /bookings/:id/checkin
func (h *Handler) CheckIn(c *gin.Context) {
	bookingID, ok := parseID(c, "id")
	if !ok {
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "authentication required", nil))
		c.Abort()
		return
	}

	renting, err := h.service.CreateFromBooking(c.Request.Context(), bookingID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, renting)
}

// List godoc
// GET /rentings
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

	rentings, err := h.service.List(c.Request.Context(), userID, role, q.Status, q.Limit, q.Offset)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, rentings)
}

// Get godoc
// GET /rentings/:id
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

	renting, err := h.service.Get(c.Request.Context(), id, userID, role)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, renting)
}

// CheckOut godoc
// PATCH /rentings/:id/checkout
func (h *Handler) CheckOut(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req checkOutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	renting, err := h.service.CheckOut(c.Request.Context(), id, req.CheckOutDate, req.PaymentMethod)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, renting)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrRentingNotFound):
		_ = c.Error(middleware.New(http.StatusNotFound, "renting not found", err))
	case errors.Is(err, ErrBookingNotFound):
		_ = c.Error(middleware.New(http.StatusNotFound, "booking not found", err))
	case errors.Is(err, ErrRoomNotFound):
		_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "room not found", err))
	case errors.Is(err, ErrCustomerNotFound):
		_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "customer not found", err))
	case errors.Is(err, ErrEmployeeNotFound):
		_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "employee profile not found for your account", err))
	case errors.Is(err, ErrRoomOccupied):
		_ = c.Error(middleware.New(http.StatusConflict, "room is already occupied or reserved for this date", err))
	case errors.Is(err, ErrBookingNotActive):
		_ = c.Error(middleware.New(http.StatusConflict, "booking is not in active status", err))
	case errors.Is(err, ErrAlreadyCheckedOut):
		_ = c.Error(middleware.New(http.StatusConflict, "renting is already completed", err))
	case errors.Is(err, ErrInvalidCheckout):
		_ = c.Error(middleware.New(http.StatusBadRequest, "check-out date must be after check-in date", err))
	case errors.Is(err, ErrInvalidPaymentMethod):
		_ = c.Error(middleware.New(http.StatusBadRequest, "invalid payment method — accepted: cash, credit_card, debit_card, bank_transfer", err))
	case errors.Is(err, ErrNotOwner):
		_ = c.Error(middleware.New(http.StatusForbidden, "you do not have access to this renting", err))
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
