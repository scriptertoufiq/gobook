package migrations

import (
	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
)

func init() {
	Register(Migration{
		ID: "20260813000001_create_users_table",

		// AutoMigrate builds the table from the struct, so internal/models/user.go
		// stays the single description of the schema. Later changes to those
		// fields need a new migration of their own — editing this one does
		// nothing on a database where it has already run.
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.User{})
		},

		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.User{})
		},
	})
}
