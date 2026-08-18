package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/go-mvc/internal/middleware"
	"github.com/scriptertoufiq/go-mvc/internal/requests"
	"github.com/scriptertoufiq/go-mvc/internal/resources"
	"github.com/scriptertoufiq/go-mvc/internal/services"
	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/cache"
	"github.com/scriptertoufiq/go-mvc/pkg/pagination"
	"github.com/scriptertoufiq/go-mvc/pkg/response"
)

type PostController struct {
	service *services.PostService
}

func NewPostController(service *services.PostService) *PostController {
	return &PostController{service: service}
}

// Index handles GET /api/v1/posts
//
// Supports the standard list query — page, per_page, search, sort_by, sort_dir —
// plus `user_id` to narrow to a single author.
func (ctrl *PostController) Index(c *gin.Context) {
	params := pagination.FromQuery(c.Query)

	// Absent or unparseable means "every author", which is the default listing.
	var authorID uint
	if raw := c.Query("user_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			response.Error(c, apperror.BadRequest("Invalid user_id parameter."))
			return
		}
		authorID = uint(parsed)
	}

	posts, meta, err := ctrl.service.List(c.Request.Context(), params, authorID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Paginated(c, resources.NewPostCollection(posts), meta)
}

// Show handles GET /api/v1/posts/:id
//
// The response says where the data came from, in two forms: an X-Cache header
// for machines, and a message in the envelope for whoever is reading the JSON.
// The header is the conventional signal — proxies and browser tools already
// understand HIT/MISS — while the message is what shows up in a terminal.
func (ctrl *PostController) Show(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	post, source, err := ctrl.service.Get(c.Request.Context(), id)
	if err != nil {
		// A miss that ends in an error was still answered by the database, but
		// there is no data to label — the error envelope stands on its own.
		response.Error(c, err)
		return
	}

	c.Header("X-Cache", cacheHeader(source))
	response.OKWithMessage(c, resources.NewPostResource(post), sourceMessage(source))
}

// cacheHeader renders the source as the conventional HIT/MISS token.
func cacheHeader(source cache.Source) string {
	if source.IsHit() {
		return "HIT"
	}
	return "MISS"
}

// sourceMessage phrases the same fact for a human reading the body.
func sourceMessage(source cache.Source) string {
	if source.IsHit() {
		return "Served from cache."
	}
	return "Served from database."
}

// Store handles POST /api/v1/posts
//
// The author comes from the token, never the body — CreatePostRequest has no
// user_id field, so publishing as somebody else is not expressible.
func (ctrl *PostController) Store(c *gin.Context) {
	var req requests.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	post, err := ctrl.service.Create(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, resources.NewPostResource(post))
}

// Update handles PATCH and PUT /api/v1/posts/:id
//
// Only the post's author may edit it — the service rejects everyone else,
// administrators included.
func (ctrl *PostController) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	var req requests.UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	callerID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	post, err := ctrl.service.Update(c.Request.Context(), id, callerID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewPostResource(post))
}

// Destroy handles DELETE /api/v1/posts/:id
//
// Only the post's author may delete it.
func (ctrl *PostController) Destroy(c *gin.Context) {
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

	response.Message(c, "Post deleted.")
}
