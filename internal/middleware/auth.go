package middleware

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/jwt"
	"github.com/scriptertoufiq/go-mvc/pkg/response"
)

// Context keys set by Auth and read by downstream handlers.
const (
	ContextUserID   = "auth_user_id"
	ContextRole     = "auth_role"
	ContextVerified = "auth_verified"
)

// Auth validates the Bearer token and stashes the identity on the context.
// Every rejection is a flat 401 with the same message — distinguishing
// "expired" from "bad signature" is free reconnaissance for an attacker.
func Auth(manager *jwt.Manager) gin.HandlerFunc {
	unauthorized := func(c *gin.Context) {
		response.Error(c, apperror.Unauthorized("Authentication required."))
	}

	return func(c *gin.Context) {
		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			unauthorized(c)
			return
		}

		claims, err := manager.Parse(raw)
		if err != nil {
			unauthorized(c)
			return
		}

		userID, err := claims.UserID()
		if err != nil {
			unauthorized(c)
			return
		}

		c.Set(ContextUserID, userID)
		c.Set(ContextRole, claims.Role)
		c.Set(ContextVerified, claims.Verified)
		c.Next()
	}
}

// RequireVerified is the email-verification gate. When the feature is switched
// off it is a pass-through, so the same route table serves both configurations.
//
// The verified flag comes from the JWT claim rather than a database read, which
// keeps protected routes at zero extra queries. The trade-off: a token minted
// before verification still reads false until it expires (JWT_ACCESS_TTL,
// 15 minutes by default). Calling /auth/refresh updates it immediately.
func RequireVerified(enabled bool) gin.HandlerFunc {
	if !enabled {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		if verified, ok := c.Get(ContextVerified); ok && verified == true {
			c.Next()
			return
		}

		response.Error(c, apperror.New(
			http.StatusForbidden,
			"email_not_verified",
			"Verify your email address to access this resource.",
		))
	}
}

// RequireRole gates a route on the caller's role. Must run after Auth.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !HasRole(c, roles...) {
			response.Error(c, apperror.Forbidden("You do not have permission to perform this action."))
			return
		}

		c.Next()
	}
}

// RequireSelfOrRole allows a request when the caller holds one of the given
// roles, or when the `param` route value is their own user id.
//
// This is the ownership check for /users/:id. Without it, any authenticated
// caller could address any other account — and since the update DTO carries
// `password`, that is account takeover, not merely an information leak.
func RequireSelfOrRole(param string, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if HasRole(c, roles...) {
			c.Next()
			return
		}

		userID, ok := CurrentUserID(c)
		if !ok {
			response.Error(c, apperror.Unauthorized("Authentication required."))
			return
		}

		target, err := strconv.ParseUint(c.Param(param), 10, 64)
		if err != nil || uint(target) != userID {
			response.Error(c, apperror.Forbidden("You may only access your own account."))
			return
		}

		c.Next()
	}
}

// HasRole reports whether the authenticated caller holds any of these roles.
// Reports false when Auth has not run, so it fails closed.
func HasRole(c *gin.Context, roles ...string) bool {
	value, exists := c.Get(ContextRole)
	if !exists {
		return false
	}

	current, ok := value.(string)
	return ok && slices.Contains(roles, current)
}

// CurrentUserID reads the authenticated user id set by Auth.
func CurrentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(ContextUserID)
	if !exists {
		return 0, false
	}
	id, ok := value.(uint)
	return id, ok
}

// bearerToken pulls the credential out of an `Authorization: Bearer <token>`
// header, case-insensitively on the scheme.
func bearerToken(header string) (string, bool) {
	scheme, credentials, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "bearer") {
		return "", false
	}

	credentials = strings.TrimSpace(credentials)
	return credentials, credentials != ""
}
