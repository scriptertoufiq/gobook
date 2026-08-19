package migrations

import (
	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
)

func init() {
	Register(Migration{
		ID: "20260818164821_create_posts_table",

		// Built from the struct, so internal/models/post.go stays the
		// single description of the schema. Editing those fields later does
		// nothing on a database where this has already run — each subsequent
		// change needs its own migration:
		//
		//	go run ./cmd/make migration add_something_to_posts
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.Post{})
		},

		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.Post{})
		},
	})
}
