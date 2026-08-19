package migrations

import (
	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
)

func init() {
	Register(Migration{
		ID: "20260819191920_create_comments_table",

		// Built from the struct, so internal/models/comment.go stays the single
		// description of the schema — including the two composite indexes that
		// keep reading a thread from needing a filesort.
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.Comment{})
		},

		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.Comment{})
		},
	})
}
