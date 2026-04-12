package customers

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// RegisterRoutes wires the customer handlers to the router.
// Read:  GET /customers, GET /customers/:id  — admin or staff
// Write: PUT /customers/:id, DELETE /customers/:id — admin only
func RegisterRoutes(r *gin.Engine, db *sql.DB, jwtSecret string) {
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	read := r.Group("/customers")
	read.Use(middleware.JWT(jwtSecret), middleware.RequireRole("admin", "staff"))
	read.GET("", h.GetAll)
	read.GET("/:id", h.GetByID)

	write := r.Group("/customers")
	write.Use(middleware.JWT(jwtSecret), middleware.RequireRole("admin"))
	write.PUT("/:id", h.Update)
	write.DELETE("/:id", h.Delete)
}
