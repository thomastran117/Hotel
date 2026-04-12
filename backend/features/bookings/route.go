package bookings

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// RegisterRoutes wires the booking handlers to the router.
//
// Guest only:    POST /bookings
// Any auth:      GET  /bookings
// Any auth:      GET  /bookings/:id
// Any auth:      DELETE /bookings/:id  (guests: own only; staff/admin: any)
func RegisterRoutes(r *gin.Engine, db *sql.DB, jwtSecret string) {
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	auth := r.Group("")
	auth.Use(middleware.JWT(jwtSecret))

	// Creating a booking is restricted to guests.
	guest := auth.Group("")
	guest.Use(middleware.RequireRole("guest"))
	guest.POST("/bookings", h.Create)

	// Listing, viewing, and cancelling are available to all authenticated roles.
	auth.GET("/bookings", h.List)
	auth.GET("/bookings/:id", h.Get)
	auth.DELETE("/bookings/:id", h.Cancel)
}
