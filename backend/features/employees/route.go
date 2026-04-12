package employees

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/middleware"
)

// RegisterRoutes wires the employee handlers to the router.
// Read:  GET /employees, GET /employees/:id  — admin or staff
// Write: PUT /employees/:id, DELETE /employees/:id — admin only
func RegisterRoutes(r *gin.Engine, db *sql.DB, jwtSecret string) {
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	// /me routes are registered first so gin matches them before /:id
	me := r.Group("/employees/me")
	me.Use(middleware.JWT(jwtSecret), middleware.RequireRole("admin", "staff"))
	me.GET("", h.GetMe)
	me.PUT("/positions", h.UpdateMyPositions)

	read := r.Group("/employees")
	read.Use(middleware.JWT(jwtSecret), middleware.RequireRole("admin", "staff"))
	read.GET("", h.GetAll)
	read.GET("/:id", h.GetByID)

	write := r.Group("/employees")
	write.Use(middleware.JWT(jwtSecret), middleware.RequireRole("admin"))
	write.PUT("/:id", h.Update)
	write.DELETE("/:id", h.Delete)
}
