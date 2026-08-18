package migrations

import (
	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
)

func init() {
	Register(Migration{
		ID: "20260813000004_create_password_reset_tokens_table",

		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.PasswordResetToken{})
		},

		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.PasswordResetToken{})
		},
	})
}
