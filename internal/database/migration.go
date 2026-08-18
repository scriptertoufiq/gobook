package database

import (
	"fmt"
	"log"
	"strconv"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/database/migrations"
)

// This file is the thin, logging wrapper the CLI talks to. The schema history
// itself lives in internal/database/migrations — one file per change, each
// registering itself. There is no list to maintain here.
//
// To add a change:
//
//	go run ./cmd/make migration add_status_to_posts
//
// then fill in Up and Down and run `make migrate`.

// Migrate applies every pending migration.
func Migrate(db *gorm.DB) error {
	pending, err := migrations.Pending(db)
	if err == nil {
		log.Printf("migrate: %d migration(s) pending of %d total", len(pending), len(migrations.All()))
	}

	applied, err := migrations.Up(db)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		log.Println("migrate: nothing to migrate")
		return nil
	}

	log.Printf("migrate: applied %d migration(s)", len(applied))
	return nil
}

// Fresh drops every table and replays the whole history. Destructive — the CLI
// guards this behind an explicit flag and refuses to run in production.
func Fresh(db *gorm.DB) error {
	applied, err := migrations.Fresh(db)
	if err != nil {
		return err
	}

	log.Printf("migrate: rebuilt from scratch, %d migration(s) applied", len(applied))
	return nil
}

// Rollback reverses the last `batches` batches of migrations. Destructive.
func Rollback(db *gorm.DB, batches int) error {
	rolled, err := migrations.Rollback(db, batches)
	if err != nil {
		return err
	}

	if len(rolled) == 0 {
		log.Println("migrate: nothing to roll back")
		return nil
	}

	log.Printf("migrate: rolled back %d migration(s)", len(rolled))
	return nil
}

// Status prints what has run and what has not.
func Status(db *gorm.DB) error {
	states, err := migrations.Status(db)
	if err != nil {
		return err
	}

	if len(states) == 0 {
		log.Println("migrate: no migrations registered")
		return nil
	}

	// Printed rather than logged: this is a report the operator asked for, not
	// a running commentary, so it does not want a timestamp on every line.
	fmt.Println()
	fmt.Printf("  %-8s  %-5s  %s\n", "STATUS", "BATCH", "MIGRATION")
	fmt.Printf("  %-8s  %-5s  %s\n", "--------", "-----", "---------")

	var pending int
	for _, s := range states {
		status, batch, note := "pending", "-", ""

		switch {
		case s.Orphaned:
			status, batch, note = "orphaned", strconv.Itoa(s.Batch), "  (file missing)"
		case s.Applied:
			status, batch = "applied", strconv.Itoa(s.Batch)
		default:
			pending++
		}

		fmt.Printf("  %-8s  %-5s  %s%s\n", status, batch, s.ID, note)
	}

	fmt.Println()
	log.Printf("migrate: %d of %d migration(s) pending", pending, len(states))
	return nil
}
