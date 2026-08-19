package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/go-mvc/internal/middleware"
	"github.com/scriptertoufiq/go-mvc/internal/requests"
	"github.com/scriptertoufiq/go-mvc/internal/resources"
	"github.com/scriptertoufiq/go-mvc/internal/services"
	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/response"
)

type ReactionController struct {
	service *services.ReactionService
}

func NewReactionController(service *services.ReactionService) *ReactionController {
	return &ReactionController{service: service}
}

// unavailable answers when reactions have no store behind them, which happens
// only with REDIS_ENABLED=false. A clear 503 beats a nil dereference, and beats
// pretending the reaction was recorded.
func (ctrl *ReactionController) unavailable(c *gin.Context) bool {
	if ctrl.service != nil {
		return false
	}

	response.Error(c, apperror.New(
		http.StatusServiceUnavailable,
		"reactions_unavailable",
		"Reactions are switched off on this server.",
	))
	return true
}

// Set handles PUT /api/v1/posts/:id/reaction
//
// PUT rather than POST because setting a reaction is idempotent — sending love
// twice leaves you with love, not two of them. Sending the reaction you already
// hold takes it back, which is how a picker's toggle behaves.
func (ctrl *ReactionController) Set(c *gin.Context) {
	if ctrl.unavailable(c) {
		return
	}

	postID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	var req requests.SetReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	summary, err := ctrl.service.Set(c.Request.Context(), postID, userID, req.Type, req.When())
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewReactionResource(summary))
}

// Remove handles DELETE /api/v1/posts/:id/reaction
//
// Idempotent: removing when nothing is held still succeeds, because the
// caller's intent is already satisfied.
func (ctrl *ReactionController) Remove(c *gin.Context) {
	if ctrl.unavailable(c) {
		return
	}

	postID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	// DELETE carries no body, so a replayed removal states its time in the
	// query string. Anything unparseable is treated as "now", which is what an
	// ordinary request means anyway.
	var actedAt time.Time
	if raw := c.Query("acted_at"); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			actedAt = parsed
		}
	}

	summary, err := ctrl.service.Remove(c.Request.Context(), postID, userID, actedAt)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewReactionResource(summary))
}

// Show handles GET /api/v1/posts/:id/reactions — the tally plus the viewer's
// own choice, for a client that wants it without re-reading the post.
func (ctrl *ReactionController) Show(c *gin.Context) {
	if ctrl.unavailable(c) {
		return
	}

	postID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	// Auth has run, so a missing id means the route was mounted wrong; treating
	// it as "no viewer" still returns a correct tally.
	viewerID, _ := middleware.CurrentUserID(c)

	summary, err := ctrl.service.Summary(c.Request.Context(), postID, viewerID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewReactionResource(summary))
}
