package rentings

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// RegisterRoutes wires the renting handlers to the router.
//
// Staff/Admin only: POST /rentings
// Staff/Admin only: POST /bookings/:id/checkin
// Staff/Admin only: PATCH /rentings/:id/checkout
// Any auth:         GET  /rentings
// Any auth:         GET  /rentings/:id
func RegisterRoutes(r *gin.Engine, db *sql.DB, jwtSecret string) {
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	auth := r.Group("")
	auth.Use(middleware.JWT(jwtSecret))

	// Mutating operations are restricted to staff and admins.
	staff := auth.Group("")
	staff.Use(middleware.RequireRole("staff", "admin"))
	staff.POST("/rentings", h.CreateWalkIn)
	staff.POST("/bookings/:id/checkin", h.CheckIn)
	staff.PATCH("/rentings/:id/checkout", h.CheckOut)

	// Read operations are available to all authenticated roles.
	auth.GET("/rentings", h.List)
	auth.GET("/rentings/:id", h.Get)
}
