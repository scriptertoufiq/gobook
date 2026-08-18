// Package migrations is the versioned schema history.
//
// Each migration is one file in this directory that registers itself from an
// init function, so adding one means adding a file — there is no central list
// to edit, and therefore nothing for two branches to conflict over. Files are
// named <timestamp>_<what_it_does>.go and applied in ID order, which is the
// order they were written.
//
// Migrations are expressed through the models wherever they can be:
//
//	Up: func(db *gorm.DB) error { return db.AutoMigrate(&models.Post{}) }
//
// That keeps the struct the single description of the schema instead of
// duplicating every column as SQL. AutoMigrate is additive and idempotent, so
// a migration written this way is safe against a database that already has the
// table — which is exactly what let this ledger be introduced over a schema
// AutoMigrate had already built.
//
// AutoMigrate never drops, narrows or renames anything, so for those reach for
// the migrator or raw SQL:
//
//	db.Migrator().DropColumn(&models.Post{}, "subtitle")
//	db.Migrator().RenameColumn(&models.Post{}, "body", "content")
//	db.Exec("UPDATE posts SET status = ? WHERE status = ''", "draft")
package migrations

import (
	"slices"
	"strings"

	"gorm.io/gorm"
)

// Migration is one reversible change to the schema.
type Migration struct {
	// ID is the file name without .go, e.g. 20260813000001_create_users_table.
	// It is the key recorded in the ledger, so it must never change once the
	// migration has run anywhere — renaming one makes it run a second time.
	ID string

	// Up applies the change.
	Up func(db *gorm.DB) error

	// Down reverses it. Leaving it nil marks the migration irreversible, and
	// Rollback then refuses the whole batch rather than unwinding part of it —
	// a half-rolled-back schema matches no version of the code.
	Down func(db *gorm.DB) error
}

// registry is populated by the init function in each migration file.
var registry = map[string]Migration{}

// Register adds a migration to the history. It is called from init, so a
// duplicate or malformed ID panics the moment the binary starts rather than
// part-way through a deploy.
func Register(m Migration) {
	switch {
	case strings.TrimSpace(m.ID) == "":
		panic("migrations: Register called with an empty ID")
	case m.Up == nil:
		panic("migrations: " + m.ID + " has no Up function")
	}

	if _, exists := registry[m.ID]; exists {
		panic("migrations: duplicate migration ID " + m.ID)
	}

	registry[m.ID] = m
}

// All returns every registered migration in the order they must be applied.
func All() []Migration {
	out := make([]Migration, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}

	slices.SortFunc(out, func(a, b Migration) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// find returns a registered migration by ID.
func find(id string) (Migration, bool) {
	m, ok := registry[id]
	return m, ok
}
