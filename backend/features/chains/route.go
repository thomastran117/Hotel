package chains

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// RegisterRoutes wires the chain handlers to the router.
// Public: GET /chains, GET /chains/:id
// Admin:  POST /chains, PUT /chains/:id, DELETE /chains/:id
func RegisterRoutes(r *gin.Engine, db *sql.DB, jwtSecret string) {
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	pub := r.Group("/chains")
	pub.GET("", h.List)
	pub.GET("/:id", h.GetByID)

	adm := r.Group("/chains")
	adm.Use(middleware.JWT(jwtSecret), middleware.RequireRole("admin"))
	adm.POST("", h.Create)
	adm.PUT("/:id", h.Update)
	adm.DELETE("/:id", h.Delete)
}
