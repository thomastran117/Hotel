package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole returns a middleware that aborts with 403 if the authenticated
// user's role is not in the allowed list. Must be used after JWT middleware.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		role, ok := GetRole(c)
		if !ok {
			_ = c.Error(New(http.StatusForbidden, "forbidden", nil))
			c.Abort()
			return
		}
		if _, ok := allowed[role]; !ok {
			_ = c.Error(New(http.StatusForbidden, "forbidden", nil))
			c.Abort()
			return
		}
		c.Next()
	}
}
