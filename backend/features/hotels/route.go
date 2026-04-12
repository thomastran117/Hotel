package hotels

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// RegisterRoutes wires the hotel handlers to the router.
// Public: GET /hotels, GET /hotels/:id
// Admin:  POST /hotels, PUT /hotels/:id, DELETE /hotels/:id
func RegisterRoutes(r *gin.Engine, db *sql.DB, jwtSecret string) {
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	pub := r.Group("/hotels")
	pub.GET("", h.Search)
	pub.GET("/:id", h.GetByID)

	adm := r.Group("/hotels")
	adm.Use(middleware.JWT(jwtSecret), middleware.RequireRole("admin"))
	adm.POST("", h.Create)
	adm.PUT("/:id", h.Update)
	adm.DELETE("/:id", h.Delete)
}
