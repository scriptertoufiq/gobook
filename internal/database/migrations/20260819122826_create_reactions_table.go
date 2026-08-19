package migrations

import (
	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
)

func init() {
	Register(Migration{
		ID: "20260819122826_create_reactions_table",

		// Built from the struct, so internal/models/reaction.go stays the single
		// description of the schema — including the unique index on
		// (post_id, user_id) that the batch upsert depends on.
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.Reaction{})
		},

		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.Reaction{})
		},
	})
}
