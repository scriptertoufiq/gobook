package controllers

import (
	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/gobook/internal/middleware"
	"github.com/scriptertoufiq/gobook/internal/requests"
	"github.com/scriptertoufiq/gobook/internal/resources"
	"github.com/scriptertoufiq/gobook/internal/services"
	"github.com/scriptertoufiq/gobook/pkg/apperror"
	"github.com/scriptertoufiq/gobook/pkg/pagination"
	"github.com/scriptertoufiq/gobook/pkg/response"
)

type CommentController struct {
	service *services.CommentService
}

func NewCommentController(service *services.CommentService) *CommentController {
	return &CommentController{service: service}
}

// Index handles GET /api/v1/posts/:id/comments — a post's top-level comments,
// each carrying how many replies it has.
func (ctrl *CommentController) Index(c *gin.Context) {
	postID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	thread, err := ctrl.service.ForPost(c.Request.Context(), postID, pagination.FromQuery(c.Query))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Paginated(c,
		resources.NewCommentCollection(thread.Comments, thread.ReplyCounts), thread.Meta)
}

// Replies handles GET /api/v1/comments/:id/replies
func (ctrl *CommentController) Replies(c *gin.Context) {
	parentID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	thread, err := ctrl.service.Replies(c.Request.Context(), parentID, pagination.FromQuery(c.Query))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Paginated(c,
		resources.NewCommentCollection(thread.Comments, thread.ReplyCounts), thread.Meta)
}

// Store handles POST /api/v1/posts/:id/comments
func (ctrl *CommentController) Store(c *gin.Context) {
	postID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	var req requests.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	comment, err := ctrl.service.Create(c.Request.Context(), postID, userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, resources.NewCommentResource(comment, 0))
}

// Reply handles POST /api/v1/comments/:id/replies
//
// A separate route rather than an optional parent_id on Store: it makes the
// two-level rule something the URL expresses, and removes any way to file a
// reply under a different post from the comment it answers.
func (ctrl *CommentController) Reply(c *gin.Context) {
	parentID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	var req requests.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	reply, err := ctrl.service.Reply(c.Request.Context(), parentID, userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, resources.NewCommentResource(reply, 0))
}

// Update handles PATCH and PUT /api/v1/comments/:id — the author only.
func (ctrl *CommentController) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	var req requests.UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	callerID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	comment, err := ctrl.service.Update(c.Request.Context(), id, callerID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewCommentResource(comment, 0))
}

// Destroy handles DELETE /api/v1/comments/:id — the author only. Deleting a
// top-level comment takes its replies with it.
func (ctrl *CommentController) Destroy(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	callerID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	if err := ctrl.service.Delete(c.Request.Context(), id, callerID); err != nil {
		response.Error(c, err)
		return
	}

	response.Message(c, "Comment deleted.")
}
