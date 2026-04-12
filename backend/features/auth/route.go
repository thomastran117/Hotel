package auth

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"hotel/application/cache"
)

// RegisterRoutes wires the auth handlers to the router under /auth.
func RegisterRoutes(r *gin.Engine, db *sql.DB, store cache.Store, jwtSecret string) {
	repo := NewRepository(db)
	service := NewService(repo, store, jwtSecret)
	handler := NewHandler(service)

	g := r.Group("/auth")
	{
		g.POST("/signup", handler.Signup)
		g.POST("/login", handler.Login)
		g.POST("/refresh", handler.Refresh)
		g.POST("/logout", handler.Logout)
	}
}
