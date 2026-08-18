// Package routes is the routing table — the file you open to answer
// "what endpoints does this service expose?".
package routes

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/go-mvc/internal/container"
	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/response"
)

// Register mounts every route on the engine.
func Register(r *gin.Engine, c *container.Container) {
	r.GET("/health", c.Health.Show)

	api := r.Group("/api/v1")
	registerAPI(api, c)

	// Gin leaves this off by default, which routes a right-path/wrong-method
	// request to NoRoute — so `GET /auth/register` reports "Route not found"
	// when the route plainly exists. Turning it on makes the NoMethod handler
	// below reachable, and the answer accurate.
	r.HandleMethodNotAllowed = true

	// Fallbacks so unknown paths still return the standard envelope.
	r.NoRoute(func(ctx *gin.Context) {
		response.Error(ctx, apperror.New(http.StatusNotFound, "not_found", fmt.Sprintf(
			"No endpoint matches %s %s. Check the path and method against the API reference.",
			ctx.Request.Method, ctx.Request.URL.Path)))
	})
	r.NoMethod(func(ctx *gin.Context) {
		response.Error(ctx, apperror.New(
			http.StatusMethodNotAllowed,
			"method_not_allowed",
			fmt.Sprintf("%s is not supported on this endpoint. Check the Allow header for the methods that are.", ctx.Request.Method),
		))
	})
}
