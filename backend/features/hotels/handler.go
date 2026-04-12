package hotels

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// Handler exposes HTTP handlers for the hotels feature.
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

type hotelRequest struct {
	ChainID    uint             `json:"chain_id"    binding:"required"`
	ManagerID  *uint            `json:"manager_id"`
	Name       string           `json:"name"        binding:"required,max=255"`
	StarRating int              `json:"star_rating" binding:"required,min=1,max=5"`
	Street     string           `json:"street"      binding:"required,max=255"`
	City       string           `json:"city"        binding:"required,max=100"`
	State      string           `json:"state"       binding:"required,max=100"`
	Country    string           `json:"country"     binding:"required,max=100"`
	Zip        string           `json:"zip"         binding:"required,max=20"`
	Contacts   []contactRequest `json:"contacts"`
}

// searchQuery holds optional query parameters for the hotel search route.
type searchQuery struct {
	ChainID  *uint    `form:"chain_id"`
	Name     *string  `form:"name"`
	City     *string  `form:"city"`
	State    *string  `form:"state"`
	Country  *string  `form:"country"`
	MinStars *int     `form:"min_stars"`
	MaxStars *int     `form:"max_stars"`
	MinRooms *int     `form:"min_rooms"`
	MaxRooms *int     `form:"max_rooms"`
	MinPrice *float64 `form:"min_price"`
	MaxPrice *float64 `form:"max_price"`
	Capacity *string  `form:"capacity"`
	CheckIn  *string  `form:"check_in"`
	CheckOut *string  `form:"check_out"`
	Limit    int      `form:"limit"`
	Offset   int      `form:"offset"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Create godoc
// POST /hotels
func (h *Handler) Create(c *gin.Context) {
	var req hotelRequest
	if !bindJSON(c, &req) {
		return
	}

	hotel, err := h.service.Create(c.Request.Context(), toInput(req))
	if err != nil {
		if errors.Is(err, ErrChainNotFound) {
			_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "chain not found", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusCreated, hotel)
}

// Search godoc
// GET /hotels
func (h *Handler) Search(c *gin.Context) {
	var q searchQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(middleware.New(http.StatusBadRequest, err.Error(), err))
		c.Abort()
		return
	}

	// Validate capacity if provided.
	if q.Capacity != nil {
		switch *q.Capacity {
		case "single", "double", "triple", "quad":
		default:
			_ = c.Error(middleware.New(http.StatusBadRequest, "capacity must be one of: single, double, triple, quad", nil))
			c.Abort()
			return
		}
	}

	// check_in and check_out must be provided together.
	if (q.CheckIn == nil) != (q.CheckOut == nil) {
		_ = c.Error(middleware.New(http.StatusBadRequest, "check_in and check_out must be provided together", nil))
		c.Abort()
		return
	}

	filter := HotelFilter{
		ChainID:  q.ChainID,
		Name:     q.Name,
		City:     q.City,
		State:    q.State,
		Country:  q.Country,
		MinStars: q.MinStars,
		MaxStars: q.MaxStars,
		MinRooms: q.MinRooms,
		MaxRooms: q.MaxRooms,
		MinPrice: q.MinPrice,
		MaxPrice: q.MaxPrice,
		Capacity: q.Capacity,
		CheckIn:  q.CheckIn,
		CheckOut: q.CheckOut,
		Limit:    q.Limit,
		Offset:   q.Offset,
	}

	hotels, err := h.service.Search(c.Request.Context(), filter)
	if err != nil {
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, hotels)
}

// GetByID godoc
// GET /hotels/:id
func (h *Handler) GetByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	hotel, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrHotelNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "hotel not found", err))
			return
		}
		_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		return
	}

	c.JSON(http.StatusOK, hotel)
}

// Update godoc
// PUT /hotels/:id
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req hotelRequest
	if !bindJSON(c, &req) {
		return
	}

	hotel, err := h.service.Update(c.Request.Context(), id, toInput(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrHotelNotFound):
			_ = c.Error(middleware.New(http.StatusNotFound, "hotel not found", err))
		case errors.Is(err, ErrChainNotFound):
			_ = c.Error(middleware.New(http.StatusUnprocessableEntity, "chain not found", err))
		default:
			_ = c.Error(middleware.New(http.StatusInternalServerError, "internal server error", err))
		}
		return
	}

	c.JSON(http.StatusOK, hotel)
}

// Delete godoc
// DELETE /hotels/:id
func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrHotelNotFound) {
			_ = c.Error(middleware.New(http.StatusNotFound, "hotel not found", err))
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

func toInput(req hotelRequest) HotelInput {
	contacts := make([]ContactInput, len(req.Contacts))
	for i, ct := range req.Contacts {
		contacts[i] = ContactInput{Type: ct.Type, Value: ct.Value}
	}
	return HotelInput{
		ChainID:    req.ChainID,
		ManagerID:  req.ManagerID,
		Name:       req.Name,
		StarRating: req.StarRating,
		Street:     req.Street,
		City:       req.City,
		State:      req.State,
		Country:    req.Country,
		Zip:        req.Zip,
		Contacts:   contacts,
	}
}
