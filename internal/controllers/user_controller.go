// Package controllers is the HTTP edge. A handler does exactly four things:
// parse input, call one service method, map the result to a resource, respond.
// If you find yourself writing an `if` about business rules here, it belongs
// in the service.
package controllers

import (
	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/gobook/internal/middleware"
	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/internal/requests"
	"github.com/scriptertoufiq/gobook/internal/resources"
	"github.com/scriptertoufiq/gobook/internal/services"
	"github.com/scriptertoufiq/gobook/pkg/apperror"
	"github.com/scriptertoufiq/gobook/pkg/pagination"
	"github.com/scriptertoufiq/gobook/pkg/response"
)

type UserController struct {
	service *services.UserService
}

func NewUserController(service *services.UserService) *UserController {
	return &UserController{service: service}
}

// Index handles GET /api/v1/users
func (ctrl *UserController) Index(c *gin.Context) {
	params := pagination.FromQuery(c.Query)

	users, meta, err := ctrl.service.List(c.Request.Context(), params)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Paginated(c, resources.NewUserCollection(users), meta)
}

// Show handles GET /api/v1/users/:id
func (ctrl *UserController) Show(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	user, err := ctrl.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewUserResource(user))
}

// Store handles POST /api/v1/users
func (ctrl *UserController) Store(c *gin.Context) {
	var req requests.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	user, err := ctrl.service.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, resources.NewUserResource(user))
}

// Update handles PATCH /api/v1/users/:id
func (ctrl *UserController) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	var req requests.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	// Ownership alone is not enough. RequireSelfOrRole lets a user edit their
	// own record — and `role` would then be a self-service path to admin, while
	// `is_active` would let anyone re-enable a suspended account. Both stay
	// administrative even on your own row.
	//
	// Rejecting is deliberate rather than silently dropping the fields: a
	// caller who thinks they changed their role should be told they did not.
	isAdmin := middleware.HasRole(c, models.RoleAdmin)
	if (req.Role != nil || req.IsActive != nil) && !isAdmin {
		response.Error(c, apperror.Forbidden("Only an administrator can change a role or account status."))
		return
	}

	// Setting a password here is administrative — an operator assigning one to
	// somebody else. Users change their own via POST /auth/password/change,
	// which demands the current password and hands back fresh tokens.
	//
	// Keeping it out of this handler is also what lets the endpoint have a
	// single response shape: returning a token payload from a PATCH that
	// usually returns a user is a contract a client cannot code against.
	// The distinction is whose account it is, not what role the caller holds.
	// An admin setting somebody else's password is administrative; an admin
	// setting their *own* is a self-service change and must prove knowledge of
	// the current one — they are the highest-value account in the system, not
	// the least protected.
	callerID, _ := middleware.CurrentUserID(c)
	if req.Password != nil && callerID == id {
		response.Error(c, apperror.Forbidden(
			"Use POST /api/v1/auth/password/change to change your own password."))
		return
	}

	user, err := ctrl.service.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewUserResource(user))
}

// Destroy handles DELETE /api/v1/users/:id
func (ctrl *UserController) Destroy(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	if err := ctrl.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}

	response.Message(c, "User deleted.")
}
