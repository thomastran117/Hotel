package reviews

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the reviews feature.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Request types ─────────────────────────────────────────────────────────────

type reviewRequest struct {
	Rating  uint8   `json:"rating"  binding:"required,min=1,max=5"`
	Comment *string `json:"comment"`
}

type listQuery struct {
	Limit  int `form:"limit"`
	Offset int `form:"offset"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// List godoc
// GET /hotels/:hotel_id/reviews
func (h *Handler) List(c *gin.Context) {
	hotelID, ok := parseID(c, "hotel_id")
	if !ok {
		return
	}

	var q listQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	reviews, err := h.service.ListByHotel(c.Request.Context(), hotelID, q.Limit, q.Offset)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, reviews)
}

// Create godoc
// POST /hotels/:hotel_id/reviews
func (h *Handler) Create(c *gin.Context) {
	hotelID, ok := parseID(c, "hotel_id")
	if !ok {
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		_ = c.Error(middleware.New(http.StatusUnauthorized, "authentication required", nil))
		c.Abort()
		return
	}

	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	review, err := h.service.Create(c.Request.Context(), ReviewInput{
		HotelID: hotelID,
		UserID:  userID,
		Rating:  req.Rating,
		Comment: req.Comment,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrHotelNotFound):
			_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "hotel not found", err))
		case errors.Is(err, ErrAlreadyReviewed):
			_ = c.Error(middleware.New(http.StatusConflict, "you have already reviewed this hotel", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	c.JSON(http.StatusCreated, review)
}

// Delete godoc
// DELETE /reviews/:id
func (h *Handler) Delete(c *gin.Context) {
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

	if err := h.service.Delete(c.Request.Context(), id, userID, role); err != nil {
		switch {
		case errors.Is(err, ErrReviewNotFound):
			_ = c.Error(middleware.New(http.StatusNotFound, "review not found", err))
		case errors.Is(err, ErrNotOwner):
			_ = c.Error(middleware.New(http.StatusForbidden, "you can only delete your own reviews", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseID(c *gin.Context, param string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, "invalid id", err))
		c.Abort()
		return 0, false
	}
	return uint(n), true
}
