package migrations

import (
	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
)

func init() {
	Register(Migration{
		ID: "20260813000003_create_email_verification_tokens_table",

		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.EmailVerificationToken{})
		},

		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.EmailVerificationToken{})
		},
	})
}
