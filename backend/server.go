package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"hotel/application/cache"
	"hotel/application/database"
	"hotel/application/environment"
	"hotel/application/middleware"
	"hotel/features/auth"
	"hotel/features/bookings"
	"hotel/features/chains"
	"hotel/features/customers"
	"hotel/features/employees"
	"hotel/features/hotels"
	"hotel/features/rentings"
	"hotel/features/reviews"
	"hotel/features/rooms"
)

func main() {
	cfg, err := environment.Load()
	if err != nil {
		log.Fatalf("startup: %v", err)
	}

	if os.Getenv("DB_RESET_SCHEMA") == "true" {
		log.Println("DB_RESET_SCHEMA=true — dropping and recreating database schema")
		if err := database.ResetSchema(cfg); err != nil {
			log.Fatalf("startup: reset schema: %v", err)
		}
		log.Println("schema reset complete")
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer db.Close()

	redisClient, err := cache.Connect(cfg)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer redisClient.Close()

	store := cache.NewStore(redisClient)

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:3040"
	}

	// gin.New() instead of gin.Default() so we control middleware order
	// explicitly: CORS → Logger → Recovery → ErrorHandler → routes.
	router := gin.New()
	router.Use(
		cors.New(cors.Config{
			AllowOrigins:     []string{corsOrigin},
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}),
		gin.Logger(),
		gin.Recovery(),
		middleware.ErrorHandler(),
	)

	auth.RegisterRoutes(router, db, store, cfg.JWTSecret)
	chains.RegisterRoutes(router, db, cfg.JWTSecret)
	hotels.RegisterRoutes(router, db, cfg.JWTSecret)
	employees.RegisterRoutes(router, db, cfg.JWTSecret)
	customers.RegisterRoutes(router, db, cfg.JWTSecret)
	rooms.RegisterRoutes(router, db, cfg.JWTSecret)
	reviews.RegisterRoutes(router, db, cfg.JWTSecret)
	bookings.RegisterRoutes(router, db, cfg.JWTSecret)
	rentings.RegisterRoutes(router, db, cfg.JWTSecret)

	// Example: protected routes use the JWT middleware on the group.
	// protected := router.Group("/api")
	// protected.Use(middleware.JWT(cfg.JWTSecret))

	log.Printf("server listening on :%s", cfg.ServerPort)
	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server: %v", err)
	}
}
