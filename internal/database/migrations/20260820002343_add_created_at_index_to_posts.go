package migrations

import (
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID: "20260820002343_add_created_at_index_to_posts",

		// Sorting a feed by created_at had no index behind it. On two million
		// rows that is a filesort of the whole table per page — measured at
		// 4.8 seconds against 0.2 for the primary key — which read from the
		// outside as "scrolling stopped loading anything".
		//
		// Composite, and deleted_at comes first because the soft-delete scope
		// puts `deleted_at IS NULL` on every query. An index on created_at
		// alone cannot serve both the filter and the sort, so MySQL would go
		// back to sorting.
		//
		// This cannot come from the model: Post takes its timestamps from the
		// shared Timestamps embed, and tagging them there would put the index
		// on every table that embeds it.
		Up: func(db *gorm.DB) error {
			return db.Exec(
				"CREATE INDEX idx_posts_deleted_created ON posts (deleted_at, created_at)").Error
		},

		Down: func(db *gorm.DB) error {
			return db.Exec("DROP INDEX idx_posts_deleted_created ON posts").Error
		},
	})
}
