package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/scriptertoufiq/go-mvc/config"
)

// Connect opens the pool and verifies it with a ping, so a bad DSN fails at
// boot instead of on the first request.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	level := logger.Warn
	if cfg.App.Debug {
		level = logger.Info
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  level,
			IgnoreRecordNotFoundError: true,
			Colorful:                  !cfg.App.IsProduction(),
		},
	)

	db, err := gorm.Open(mysql.Open(cfg.DB.DSN()), &gorm.Config{
		Logger:                                   gormLogger,
		SkipDefaultTransaction:                   true, // we open transactions explicitly where needed
		TranslateError:                           true, // driver errors -> gorm.ErrDuplicatedKey etc.
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("database: open connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: access underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.DB.ConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping %s:%s: %w", cfg.DB.Host, cfg.DB.Port, err)
	}

	return db, nil
}

// Close drains the pool on shutdown.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
