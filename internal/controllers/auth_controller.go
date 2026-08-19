package controllers

import (
	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/gobook/internal/middleware"
	"github.com/scriptertoufiq/gobook/internal/requests"
	"github.com/scriptertoufiq/gobook/internal/resources"
	"github.com/scriptertoufiq/gobook/internal/services"
	"github.com/scriptertoufiq/gobook/pkg/apperror"
	"github.com/scriptertoufiq/gobook/pkg/response"
)

type AuthController struct {
	auth  *services.AuthService
	users *services.UserService
}

func NewAuthController(auth *services.AuthService, users *services.UserService) *AuthController {
	return &AuthController{auth: auth, users: users}
}

// Register handles POST /api/v1/auth/register
func (ctrl *AuthController) Register(c *gin.Context) {
	var req requests.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	user, err := ctrl.auth.Register(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, resources.NewUserResource(user))
}

// Login handles POST /api/v1/auth/login
func (ctrl *AuthController) Login(c *gin.Context) {
	var req requests.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	pair, err := ctrl.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewTokenResource(pair))
}

// Refresh handles POST /api/v1/auth/refresh — rotates the pair.
func (ctrl *AuthController) Refresh(c *gin.Context) {
	var req requests.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	pair, err := ctrl.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewTokenResource(pair))
}

// Logout handles POST /api/v1/auth/logout. With `"all": true` it revokes every
// session; otherwise it revokes the supplied refresh token only.
func (ctrl *AuthController) Logout(c *gin.Context) {
	var req requests.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	var err error
	if req.All {
		err = ctrl.auth.LogoutAll(c.Request.Context(), userID)
	} else {
		err = ctrl.auth.Logout(c.Request.Context(), userID, req.RefreshToken)
	}
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Message(c, "Logged out.")
}

// ChangePassword handles POST /api/v1/auth/password/change — a signed-in user
// replacing their own password. Always requires the current one, and always
// returns a fresh token pair.
func (ctrl *AuthController) ChangePassword(c *gin.Context) {
	var req requests.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	pair, err := ctrl.auth.ChangePassword(c.Request.Context(), userID, req.CurrentPassword, req.Password)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewTokenResource(pair))
}

// ForgotPassword handles POST /api/v1/auth/password/forgot.
//
// The response is identical whether or not the address exists — this endpoint
// is unauthenticated, so anything else would let anyone test which emails have
// accounts.
func (ctrl *AuthController) ForgotPassword(c *gin.Context) {
	var req requests.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	if err := ctrl.auth.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		response.Error(c, err)
		return
	}

	response.Message(c, "If that email address has an account, a reset link is on its way.")
}

// ResetPassword handles POST /api/v1/auth/password/reset — the form on your
// frontend posts here with the token from the emailed link.
func (ctrl *AuthController) ResetPassword(c *gin.Context) {
	var req requests.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	if err := ctrl.auth.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		response.Error(c, err)
		return
	}

	response.Message(c, "Password updated. All sessions have been signed out — please log in again.")
}

// Me handles GET /api/v1/auth/me — the caller's own profile. Reachable while
// unverified, so a pending user can still see their state.
func (ctrl *AuthController) Me(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	user, err := ctrl.users.Get(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.NewUserResource(user))
}

// VerifyEmail handles GET /api/v1/auth/verify-email?token=... — the target of
// the emailed link, so it is a GET and takes the token from the query string.
func (ctrl *AuthController) VerifyEmail(c *gin.Context) {
	user, err := ctrl.auth.VerifyEmail(c.Request.Context(), c.Query("token"))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"message": "Email address verified.",
		"user":    resources.NewUserResource(user),
	})
}

// ResendVerification handles POST /api/v1/auth/email/resend. It sits behind
// Auth so the caller can only ever trigger mail to their own address — a public
// version would be both an enumeration oracle and a spam relay.
func (ctrl *AuthController) ResendVerification(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("Authentication required."))
		return
	}

	if err := ctrl.auth.ResendVerification(c.Request.Context(), userID); err != nil {
		response.Error(c, err)
		return
	}

	response.Message(c, "Verification email sent.")
}
