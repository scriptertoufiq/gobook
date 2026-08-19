package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/pkg/apperror"
	"github.com/scriptertoufiq/gobook/pkg/cache"
	"github.com/scriptertoufiq/gobook/pkg/response"
)

type HealthController struct {
	db    *gorm.DB
	cache cache.Cache
	app   string
}

func NewHealthController(db *gorm.DB, store cache.Cache, appName string) *HealthController {
	return &HealthController{db: db, cache: store, app: appName}
}

// Show handles GET /health — a real readiness probe that pings its
// dependencies rather than just proving the process is alive.
func (ctrl *HealthController) Show(c *gin.Context) {
	sqlDB, err := ctrl.db.DB()
	if err != nil {
		response.Error(c, apperror.Internal(err))
		return
	}

	if err := sqlDB.PingContext(c.Request.Context()); err != nil {
		response.Error(c, apperror.New(503, "database_unavailable", "Database is unreachable.").Wrap(err))
		return
	}

	// The cache is reported, not enforced. Losing Redis costs speed, not
	// correctness, so it must not take a healthy instance out of the load
	// balancer — the database, which the app genuinely cannot serve without,
	// still does.
	response.OK(c, gin.H{
		"app":      ctrl.app,
		"status":   "ok",
		"database": "connected",
		"cache":    ctrl.cacheStatus(c),
	})
}

func (ctrl *HealthController) cacheStatus(c *gin.Context) string {
	if _, disabled := ctrl.cache.(cache.Null); disabled {
		return "disabled"
	}
	if err := ctrl.cache.Ping(c.Request.Context()); err != nil {
		return "unreachable"
	}
	return "connected"
}
