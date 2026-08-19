package middleware

import (
	"fmt"
	"log"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/gobook/pkg/apperror"
	"github.com/scriptertoufiq/gobook/pkg/response"
)

// Recovery converts a panic into the standard 500 envelope instead of letting
// Gin write a bare stack trace to the client.
func Recovery(debugMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				log.Printf("panic: %v req=%s\n%s", rec, c.GetString(RequestIDKey), stack)

				err := apperror.Internal(fmt.Errorf("panic: %v", rec))
				if debugMode {
					err = apperror.New(500, "panic", fmt.Sprintf("%v", rec))
				}
				response.Error(c, err)
			}
		}()

		c.Next()
	}
}
