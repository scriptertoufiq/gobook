// Package container is the composition root: the one place where concrete
// types are wired together. Everything else depends on interfaces or is handed
// its collaborators, which is what keeps the layers testable.
//
// It lives in its own package (rather than in app) so routes can depend on it
// without creating an import cycle.
package container

import (
	"sync"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/config"
	"github.com/scriptertoufiq/go-mvc/internal/controllers"
	"github.com/scriptertoufiq/go-mvc/internal/repositories"
	"github.com/scriptertoufiq/go-mvc/internal/services"
	"github.com/scriptertoufiq/go-mvc/pkg/jwt"
	"github.com/scriptertoufiq/go-mvc/pkg/mailer"
	"github.com/scriptertoufiq/go-mvc/pkg/ratelimit"
)

// Lines marked `codegen:` are insertion points for `go run ./cmd/make`.
// Keep them — the generator writes directly above each one.
type Container struct {
	Health *controllers.HealthController
	Auth   *controllers.AuthController
	User   *controllers.UserController
	Post   *controllers.PostController
	// codegen:fields

	// JWT is exposed so the route table can build auth middleware from it.
	JWT *jwt.Manager
	// RequireEmailVerification mirrors the config flag for the same reason.
	RequireEmailVerification bool

	// Throttles. Nil when RATE_LIMIT_ENABLED=false, which the middleware
	// treats as a pass-through.
	APIRateLimiter  *ratelimit.Limiter
	AuthRateLimiter *ratelimit.Limiter

	stopBackground chan struct{}
	closeOnce      sync.Once
}

// Close stops every background goroutine: the rate-limiter janitors and the
// expired-token sweeper. Called by app.Run during graceful shutdown, and safe
// to call more than once.
func (c *Container) Close() {
	c.closeOnce.Do(func() {
		close(c.stopBackground)

		for _, l := range []*ratelimit.Limiter{c.APIRateLimiter, c.AuthRateLimiter} {
			if l != nil {
				l.Stop()
			}
		}
	})
}

// Build assembles the dependency graph bottom-up: repositories -> services ->
// controllers. Adding a resource means adding three lines here.
func Build(db *gorm.DB, cfg *config.Config) *Container {
	// Infrastructure
	jwtManager := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL)

	var apiLimiter, authLimiter *ratelimit.Limiter
	if cfg.RateLimit.Enabled {
		apiLimiter = ratelimit.New(cfg.RateLimit.Requests, cfg.RateLimit.Window)
		authLimiter = ratelimit.New(cfg.RateLimit.AuthRequests, cfg.RateLimit.AuthWindow)

		// The janitors bound memory usage — see ratelimit.StartJanitor.
		apiLimiter.StartJanitor(cfg.RateLimit.Window)
		authLimiter.StartJanitor(cfg.RateLimit.AuthWindow)
	}

	smtpMailer := mailer.NewSMTP(
		cfg.Mail.Addr(), cfg.Mail.Username, cfg.Mail.Password,
		cfg.Mail.FromAddress, cfg.Mail.FromName,
	)

	// Repositories
	userRepo := repositories.NewUserRepository(db)
	refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
	verificationRepo := repositories.NewEmailVerificationRepository(db)
	passwordResetRepo := repositories.NewPasswordResetRepository(db)
	postRepo := repositories.NewPostRepository(db)
	// codegen:repositories

	// Services
	userService := services.NewUserService(userRepo, refreshTokenRepo)
	authService := services.NewAuthService(
		userService, userRepo, refreshTokenRepo, verificationRepo, passwordResetRepo,
		jwtManager, smtpMailer,
		services.AuthOptions{
			AppName:                  cfg.App.Name,
			AppURL:                   cfg.App.URL,
			RequireEmailVerification: cfg.Auth.RequireEmailVerification,
			VerificationTTL:          cfg.Auth.VerificationTTL,
			RefreshTTL:               cfg.JWT.RefreshTTL,
			PasswordResetTTL:         cfg.Auth.PasswordResetTTL,
			PasswordResetURL:         cfg.Auth.PasswordResetURL,
		},
	)
	// AuthService depends on UserService, so the reverse direction is a
	// callback rather than a constructor argument.
	userService.OnEmailNeedsVerification(authService.HandleEmailNeedsVerification)

	postService := services.NewPostService(postRepo)
	// codegen:services

	// Background jobs
	stopBackground := make(chan struct{})
	startTokenSweeper(refreshTokenRepo, stopBackground)

	// Controllers
	return &Container{
		Health: controllers.NewHealthController(db, cfg.App.Name),
		Auth:   controllers.NewAuthController(authService, userService),
		User:   controllers.NewUserController(userService),
		Post:   controllers.NewPostController(postService),
		// codegen:controllers

		JWT:                      jwtManager,
		RequireEmailVerification: cfg.Auth.RequireEmailVerification,
		APIRateLimiter:           apiLimiter,
		AuthRateLimiter:          authLimiter,
		stopBackground:           stopBackground,
	}
}
