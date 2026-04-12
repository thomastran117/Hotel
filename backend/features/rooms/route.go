package rooms

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// RegisterRoutes wires the room handlers to the router.
// Public: GET /rooms, GET /rooms/:id
// Admin:  POST /rooms, PUT /rooms/:id, DELETE /rooms/:id
func RegisterRoutes(r *gin.Engine, db *sql.DB, jwtSecret string) {
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	pub := r.Group("/rooms")
	pub.GET("", h.Search)
	pub.GET("/:id", h.GetByID)

	adm := r.Group("/rooms")
	adm.Use(middleware.JWT(jwtSecret), middleware.RequireRole("admin"))
	adm.POST("", h.Create)
	adm.PUT("/:id", h.Update)
	adm.DELETE("/:id", h.Delete)
}
