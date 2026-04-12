package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Context keys used to store authenticated user data.
// Use GetUserID and GetRole rather than accessing these directly.
const (
	ctxKeyUserID = "auth_user_id"
	ctxKeyRole   = "auth_role"
)

// JWT returns a middleware that validates a Bearer token in the Authorization
// header. On success it stores the user ID and role in the gin context and
// calls c.Next(). On failure it attaches an AppError and aborts the chain so
// ErrorHandler can format the response.
//
// Apply this middleware to route groups that require authentication:
//
//	protected := r.Group("/api")
//	protected.Use(middleware.JWT(cfg.JWTSecret))
func JWT(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			_ = c.Error(New(http.StatusUnauthorized, "missing or invalid authorization header", nil))
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			_ = c.Error(New(http.StatusUnauthorized, "invalid or expired token", err))
			c.Abort()
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			_ = c.Error(New(http.StatusUnauthorized, "malformed token claims", nil))
			c.Abort()
			return
		}
		role, ok := claims["role"].(string)
		if !ok {
			_ = c.Error(New(http.StatusUnauthorized, "malformed token claims", nil))
			c.Abort()
			return
		}

		c.Set(ctxKeyUserID, uint(userIDFloat))
		c.Set(ctxKeyRole, role)
		c.Next()
	}
}

// GetUserID returns the authenticated user's ID from the gin context.
// Returns (0, false) if the JWT middleware has not run or authentication failed.
func GetUserID(c *gin.Context) (uint, bool) {
	v, exists := c.Get(ctxKeyUserID)
	if !exists {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok
}

// GetRole returns the authenticated user's role from the gin context.
// Returns ("", false) if the JWT middleware has not run or authentication failed.
func GetRole(c *gin.Context) (string, bool) {
	v, exists := c.Get(ctxKeyRole)
	if !exists {
		return "", false
	}
	role, ok := v.(string)
	return role, ok
}
