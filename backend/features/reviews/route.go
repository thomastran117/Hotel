package reviews

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// RegisterRoutes wires the review handlers to the router.
// Public:        GET  /reviews/hotels/:hotel_id
// Authenticated: POST /reviews/hotels/:hotel_id
// Authenticated: DELETE /reviews/:id  (owner or admin)
func RegisterRoutes(r *gin.Engine, db *sql.DB, jwtSecret string) {
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	pub := r.Group("/reviews/hotels/:hotel_id")
	pub.GET("", h.List)

	auth := r.Group("/reviews/hotels/:hotel_id")
	auth.Use(middleware.JWT(jwtSecret))
	auth.POST("", h.Create)

	authed := r.Group("/reviews")
	authed.Use(middleware.JWT(jwtSecret))
	authed.DELETE("/:id", h.Delete)
}
