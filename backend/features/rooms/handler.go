package rooms

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the rooms feature.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Request / response types ──────────────────────────────────────────────────

type damageRequest struct {
	Description string `json:"description" binding:"required"`
	Resolved    bool   `json:"resolved"`
}

type roomRequest struct {
	HotelID       uint            `json:"hotel_id"        binding:"required"`
	RoomNumber    string          `json:"room_number"     binding:"required,max=20"`
	PricePerNight float64         `json:"price_per_night" binding:"required,gt=0"`
	Capacity      string          `json:"capacity"        binding:"required,oneof=single double triple quad"`
	View          string          `json:"view"            binding:"required,oneof=none sea mountain city"`
	Extendable    bool            `json:"extendable"`
	Status        string          `json:"status"          binding:"required,oneof=available occupied maintenance"`
	Amenities     []string        `json:"amenities"`
	Damages       []damageRequest `json:"damages"`
}

// searchQuery holds optional query parameters for the room search route.
type searchQuery struct {
	HotelID    *uint    `form:"hotel_id"`
	ChainName  *string  `form:"chain_name"`
	RoomNumber *string  `form:"room_number"`
	Capacity   *string  `form:"capacity"`
	View       *string  `form:"view"`
	MinPrice   *float64 `form:"min_price"`
	MaxPrice   *float64 `form:"max_price"`
	Status     *string  `form:"status"`
	Extendable *bool    `form:"extendable"`
	CheckIn    *string  `form:"check_in"`
	CheckOut   *string  `form:"check_out"`
	Limit      int      `form:"limit"`
	Offset     int      `form:"offset"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Create godoc
// POST /rooms
func (h *Handler) Create(c *gin.Context) {
	var req roomRequest
	if !bindJSON(c, &req) {
		return
	}

	room, err := h.service.Create(c.Request.Context(), toInput(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrHotelNotFound):
			_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "hotel not found", err))
		case errors.Is(err, ErrRoomNumberTaken):
			_ = c.Error(middleware.New(http.StatusConflict, "room number already exists in this hotel", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	c.JSON(http.StatusCreated, room)
}

// Search godoc
// GET /rooms
func (h *Handler) Search(c *gin.Context) {
	var q searchQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	if q.Capacity != nil {
		switch *q.Capacity {
		case "single", "double", "triple", "quad":
		default:
			_ = c.Error(middleware.New(http.StatusBadRequest, "capacity must be one of: single, double, triple, quad", nil))
			c.Abort()
			return
		}
	}
	if q.View != nil {
		switch *q.View {
		case "none", "sea", "mountain", "city":
		default:
			_ = c.Error(middleware.New(http.StatusBadRequest, "view must be one of: none, sea, mountain, city", nil))
			c.Abort()
			return
		}
	}
	if q.Status != nil {
		switch *q.Status {
		case "available", "occupied", "maintenance":
		default:
			_ = c.Error(middleware.New(http.StatusBadRequest, "status must be one of: available, occupied, maintenance", nil))
			c.Abort()
			return
		}
	}

	// check_in and check_out must be provided together if either is given.
	if (q.CheckIn == nil) != (q.CheckOut == nil) {
		_ = c.Error(middleware.New(http.StatusBadRequest, "check_in and check_out must be provided together", nil))
		c.Abort()
		return
	}

	filter := RoomFilter{
		HotelID:    q.HotelID,
		ChainName:  q.ChainName,
		RoomNumber: q.RoomNumber,
		Capacity:   q.Capacity,
		View:       q.View,
		MinPrice:   q.MinPrice,
		MaxPrice:   q.MaxPrice,
		Status:     q.Status,
		Extendable: q.Extendable,
		CheckIn:    q.CheckIn,
		CheckOut:   q.CheckOut,
		Limit:      q.Limit,
		Offset:     q.Offset,
	}

	rooms, err := h.service.Search(c.Request.Context(), filter)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, rooms)
}

// GetByID godoc
// GET /rooms/:id
func (h *Handler) GetByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	room, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "room not found", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, room)
}

// Update godoc
// PUT /rooms/:id
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req roomRequest
	if !bindJSON(c, &req) {
		return
	}

	room, err := h.service.Update(c.Request.Context(), id, toInput(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrRoomNotFound):
			_ = c.Error(middleware.New(http.StatusNotFound, "room not found", err))
		case errors.Is(err, ErrHotelNotFound):
			_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "hotel not found", err))
		case errors.Is(err, ErrRoomNumberTaken):
			_ = c.Error(middleware.New(http.StatusConflict, "room number already exists in this hotel", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	c.JSON(http.StatusOK, room)
}

// Delete godoc
// DELETE /rooms/:id
func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "room not found", err))
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

func toInput(req roomRequest) RoomInput {
	damages := make([]DamageInput, len(req.Damages))
	for i, d := range req.Damages {
		damages[i] = DamageInput{Description: d.Description, Resolved: d.Resolved}
	}
	return RoomInput{
		HotelID:       req.HotelID,
		RoomNumber:    req.RoomNumber,
		PricePerNight: req.PricePerNight,
		Capacity:      req.Capacity,
		View:          req.View,
		Extendable:    req.Extendable,
		Status:        req.Status,
		Amenities:     req.Amenities,
		Damages:       damages,
	}
}
