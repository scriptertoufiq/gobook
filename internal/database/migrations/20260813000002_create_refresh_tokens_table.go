package migrations

import (
	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
)

func init() {
	Register(Migration{
		ID: "20260813000002_create_refresh_tokens_table",

		// After users: the model carries a foreign key to it, and AutoMigrate
		// creates constraints as it goes.
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.RefreshToken{})
		},

		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.RefreshToken{})
		},
	})
}
