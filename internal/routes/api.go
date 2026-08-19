package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/gobook/internal/container"
	"github.com/scriptertoufiq/gobook/internal/middleware"
	"github.com/scriptertoufiq/gobook/internal/models"
)

// registerAPI wires the /api/v1 group. Each resource gets a RESTful block:
//
//	GET    /users      -> Index
//	POST   /users      -> Store
//	GET    /users/:id  -> Show
//	PATCH  /users/:id  -> Update
//	DELETE /users/:id  -> Destroy
func registerAPI(api *gin.RouterGroup, c *container.Container) {
	// authenticated gates on a valid access token; verified additionally
	// requires a confirmed email — and is a pass-through when the feature is
	// switched off, so this table serves both configurations unchanged.
	authenticated := middleware.Auth(c.JWT)
	verified := middleware.RequireVerified(c.RequireEmailVerification)

	// The general throttle covers every route in this group. The auth group
	// then stacks a much tighter one on top, because those endpoints send mail
	// and run bcrypt on behalf of unauthenticated callers.
	api.Use(middleware.RateLimit(c.APIRateLimiter))

	auth := api.Group("/auth", middleware.RateLimit(c.AuthRateLimiter))
	{
		auth.POST("/register", c.Auth.Register)
		auth.POST("/login", c.Auth.Login)
		auth.POST("/refresh", c.Auth.Refresh)
		auth.GET("/verify-email", c.Auth.VerifyEmail) // target of the emailed link

		// Password recovery. Necessarily public — someone who has lost their
		// password cannot authenticate to ask for a new one.
		auth.POST("/password/forgot", c.Auth.ForgotPassword)
		auth.POST("/password/reset", c.Auth.ResetPassword)

		// Signed in, but not necessarily verified — a pending user still needs
		// to read their own state and request a new link.
		session := auth.Group("", authenticated)
		{
			session.GET("/me", c.Auth.Me)
			session.POST("/logout", c.Auth.Logout)
			session.POST("/email/resend", c.Auth.ResendVerification)
			// Reachable while unverified: a user must always be able to secure
			// their own account.
			session.POST("/password/change", c.Auth.ChangePassword)
		}
	}

	// Every route here is authorised twice: authentication proves *who* you
	// are, and one of these proves you may act on *this* record. Being signed
	// in is never on its own enough to touch someone else's account.
	admin := middleware.RequireRole(models.RoleAdmin)
	selfOrAdmin := middleware.RequireSelfOrRole("id", models.RoleAdmin)

	users := api.Group("/users", authenticated, verified)
	{
		users.GET("", admin, c.User.Index) // listing exposes every email address
		users.POST("", admin, c.User.Store)
		users.GET("/:id", selfOrAdmin, c.User.Show)
		users.PATCH("/:id", selfOrAdmin, c.User.Update)
		users.PUT("/:id", selfOrAdmin, c.User.Update)
		users.DELETE("/:id", admin, c.User.Destroy)
	}

	// Posts. Any verified user may publish; editing and deleting are limited
	// to the author, or an admin, which PostService enforces after loading the
	// row — `:id` here is a post id, so route middleware cannot judge ownership.
	posts := api.Group("/posts", authenticated, verified)
	{
		posts.GET("", c.Post.Index)
		posts.POST("", c.Post.Store)
		posts.GET("/:id", c.Post.Show)
		posts.PATCH("/:id", c.Post.Update)
		posts.PUT("/:id", c.Post.Update)
		posts.DELETE("/:id", c.Post.Destroy)

		// Reactions. Anyone signed in may react to any post — unlike editing,
		// which is the author's alone. PUT because setting one is idempotent.
		posts.GET("/:id/reactions", c.Reaction.Show)
		posts.PUT("/:id/reaction", c.Reaction.Set)
		posts.DELETE("/:id/reaction", c.Reaction.Remove)

		// Comments on a post. Replies hang off a comment, not a post, so they
		// live under /comments below.
		posts.GET("/:id/comments", c.Comment.Index)
		posts.POST("/:id/comments", c.Comment.Store)
	}

	// Comments are addressed by their own id once they exist, so editing,
	// deleting and replying do not need the post in the path — and cannot
	// disagree with it.
	comments := api.Group("/comments", authenticated, verified)
	{
		comments.GET("/:id/replies", c.Comment.Replies)
		comments.POST("/:id/replies", c.Comment.Reply)
		comments.PATCH("/:id", c.Comment.Update)
		comments.PUT("/:id", c.Comment.Update)
		comments.DELETE("/:id", c.Comment.Destroy)
	}

	// codegen:routes
	//
	// Scaffolded resources land here. `go run ./cmd/make` emits them behind
	// `authenticated, verified` — secure by default, because forgetting to add
	// a guard is silent while deleting one is a deliberate act.
}
