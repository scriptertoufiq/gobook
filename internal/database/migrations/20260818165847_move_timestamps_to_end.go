package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// timestampColumns are moved as a group, in this order.
var timestampColumns = []string{"created_at", "updated_at", "deleted_at"}

// tablesToReorder are the tables that existed before Timestamps was split out
// of Base. Tables created afterwards already have the right order.
var tablesToReorder = []string{
	"users",
	"refresh_tokens",
	"email_verification_tokens",
	"password_reset_tokens",
	"posts",
}

func init() {
	Register(Migration{
		ID: "20260818165847_move_timestamps_to_end",

		// AutoMigrate cannot express this: it adds and widens columns but never
		// repositions one, so a table built before models.Timestamps was split
		// out of Base keeps created_at/updated_at/deleted_at sitting directly
		// after id. This rewrites the position in place with ALTER TABLE, which
		// preserves every row — the alternative, dropping and rebuilding, would
		// take the data with it.
		//
		// Column order is presentation only. Nothing queries by ordinal
		// position, so this changes how `DESCRIBE` and `SELECT *` read and
		// nothing else.
		Up: func(db *gorm.DB) error {
			return reorderTimestamps(db, tablesToReorder, true)
		},

		Down: func(db *gorm.DB) error {
			return reorderTimestamps(db, tablesToReorder, false)
		},
	})
}

// reorderTimestamps moves the timestamp columns to the end of each table when
// toEnd is set, or back to directly after `id` when it is not.
func reorderTimestamps(db *gorm.DB, tables []string, toEnd bool) error {
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			continue // nothing to reorder; a fresh database builds it correctly
		}

		anchor, err := anchorColumn(db, table, toEnd)
		if err != nil {
			return err
		}
		if anchor == "" {
			continue
		}

		// Each column is placed after the previous one, so the group lands in
		// the intended order rather than reversed.
		previous := anchor
		for _, column := range timestampColumns {
			definition, err := columnDefinition(db, table, column)
			if err != nil {
				return err
			}
			if definition == "" {
				continue // column absent — nothing to move
			}

			sql := fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s AFTER `%s`",
				table, column, definition, previous)
			if err := db.Exec(sql).Error; err != nil {
				return fmt.Errorf("reordering %s.%s: %w", table, column, err)
			}
			previous = column
		}
	}

	return nil
}

// anchorColumn is the column the timestamp group is placed after: the last
// non-timestamp column when moving to the end, or the primary key when moving
// back.
func anchorColumn(db *gorm.DB, table string, toEnd bool) (string, error) {
	if !toEnd {
		return "id", nil
	}

	var name string
	err := db.Raw(`
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = ?
		  AND column_name NOT IN (?, ?, ?)
		ORDER BY ordinal_position DESC
		LIMIT 1`,
		table, timestampColumns[0], timestampColumns[1], timestampColumns[2],
	).Scan(&name).Error
	if err != nil {
		return "", fmt.Errorf("finding last column of %s: %w", table, err)
	}
	return name, nil
}

// columnDefinition reads the column's current type and nullability, so the
// MODIFY that repositions it cannot accidentally redefine it.
func columnDefinition(db *gorm.DB, table, column string) (string, error) {
	var row struct {
		ColumnType string
		IsNullable string
	}

	err := db.Raw(`
		SELECT column_type AS column_type, is_nullable AS is_nullable
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		table, column,
	).Scan(&row).Error
	if err != nil {
		return "", fmt.Errorf("reading definition of %s.%s: %w", table, column, err)
	}
	if row.ColumnType == "" {
		return "", nil
	}

	if row.IsNullable == "YES" {
		return row.ColumnType + " NULL", nil
	}
	return row.ColumnType + " NOT NULL", nil
}
