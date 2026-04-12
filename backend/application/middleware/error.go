package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError is a structured error that carries an HTTP status code and a
// user-safe message. Handlers attach it to the request context via c.Error(err)
// and return; ErrorHandler formats it into a JSON response.
type AppError struct {
	Status  int
	Message string
	Err     error // underlying cause (logged, not exposed to the client)
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }

// New constructs an AppError.
func New(status int, message string, cause error) *AppError {
	return &AppError{Status: status, Message: message, Err: cause}
}

// ErrorHandler is a global Gin middleware that converts any AppError attached
// to the context into a consistent JSON error response. It must be registered
// before route handlers so that it wraps them via c.Next().
//
// Usage in a handler:
//
//	_ = c.Error(middleware.New(http.StatusConflict, "email already in use", err))
//	return
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// If a response was already written (success path) or there are no
		// errors, there is nothing to do.
		if c.Writer.Written() || len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err
		var appErr *AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.Status, gin.H{"error": appErr.Message})
			return
		}

		// Unknown error — never leak internal details to the client.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
