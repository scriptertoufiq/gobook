// Package app boots the HTTP application: config -> database -> container ->
// router -> server, plus graceful shutdown.
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/config"
	"github.com/scriptertoufiq/go-mvc/internal/container"
	"github.com/scriptertoufiq/go-mvc/internal/database"
	"github.com/scriptertoufiq/go-mvc/internal/middleware"
	"github.com/scriptertoufiq/go-mvc/internal/routes"
	"github.com/scriptertoufiq/go-mvc/pkg/response"
)

type App struct {
	cfg       *config.Config
	db        *gorm.DB
	engine    *gin.Engine
	container *container.Container
}

// New builds a fully wired application.
func New() (*App, error) {
	cfg := config.Load()

	// Fail fast on a configuration the app cannot run safely under — chiefly a
	// missing or weak JWT signing key.
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if cfg.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		return nil, err
	}

	// Report validation problems using the field names the client actually
	// sent (`refresh_token`), not Go struct names (`RefreshToken`).
	response.UseJSONFieldNames()

	engine := gin.New()

	// Decide whose X-Forwarded-For we believe. Gin trusts every proxy by
	// default, which would let any caller spoof their address — and the rate
	// limiter keys anonymous callers by address, so that alone would render it
	// decorative. Empty TRUSTED_PROXIES means trust none and use the socket's
	// remote address, which is correct unless you sit behind a load balancer.
	if err := engine.SetTrustedProxies(cfg.App.TrustedProxies); err != nil {
		return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}

	engine.Use(
		middleware.RequestID(),
		middleware.Logger(),
		middleware.Recovery(cfg.App.Debug),
		middleware.CORS(cfg.App.CORSAllowedOrigins),
	)

	c := container.Build(db, cfg)
	routes.Register(engine, c)

	if cfg.RateLimit.Enabled {
		log.Printf("rate limiting: %d req/%s general, %d req/%s on /auth",
			cfg.RateLimit.Requests, cfg.RateLimit.Window,
			cfg.RateLimit.AuthRequests, cfg.RateLimit.AuthWindow)
	}

	return &App{cfg: cfg, db: db, engine: engine, container: c}, nil
}

// Run serves until SIGINT/SIGTERM, then drains in-flight requests before
// closing the database pool.
func (a *App) Run() error {
	srv := &http.Server{
		Addr:              ":" + a.cfg.App.Port,
		Handler:           a.engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Runs on every exit path, including a failed bind. This used to be
	// straight-line code after Shutdown, which a log.Fatalf in the serving
	// goroutine skipped entirely — os.Exit runs no deferred functions, so a
	// port collision leaked the janitors and the connection pool.
	defer a.releaseResources()

	// Buffered so the goroutine can report and exit even if nobody is selecting
	// any more, rather than blocking forever.
	serverErr := make(chan error, 1)

	go func() {
		log.Printf("%s listening on http://localhost:%s (env=%s)", a.cfg.App.Name, a.cfg.App.Port, a.cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Whichever comes first: the server giving up, or an operator asking it to.
	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	case <-quit:
		log.Println("shutting down...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}

	log.Println("stopped cleanly")
	return nil
}

// releaseResources stops background workers and drains the connection pool.
// Failures are logged rather than returned: this runs during teardown, and a
// close error must not mask whatever actually brought the server down.
func (a *App) releaseResources() {
	a.container.Close() // rate-limiter janitors, token sweeper

	if err := database.Close(a.db); err != nil {
		log.Printf("shutdown: closing database: %v", err)
	}
}

// DB exposes the connection for CLI commands and tests.
func (a *App) DB() *gorm.DB { return a.db }

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
