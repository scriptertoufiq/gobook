package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/response"
)

type HealthController struct {
	db  *gorm.DB
	app string
}

func NewHealthController(db *gorm.DB, appName string) *HealthController {
	return &HealthController{db: db, app: appName}
}

// Show handles GET /health — a real readiness probe that pings the database
// rather than just proving the process is alive.
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

	response.OK(c, gin.H{
		"app":      ctrl.app,
		"status":   "ok",
		"database": "connected",
	})
}
