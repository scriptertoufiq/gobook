// Command migrate applies schema changes and seeds.
//
//	go run ./cmd/migrate                      # apply pending migrations
//	go run ./cmd/migrate -status              # list what has run and what has not
//	go run ./cmd/migrate -seed                # migrate, then seed
//	go run ./cmd/migrate -rollback            # reverse the last batch
//	go run ./cmd/migrate -rollback -steps 3   # reverse the last three batches
//	go run ./cmd/migrate -fresh               # DROP every table, then migrate
//	go run ./cmd/migrate -fresh -seed
//
// The migrations themselves live in internal/database/migrations — one
// self-registering file per change. Create one with:
//
//	go run ./cmd/make migration add_status_to_posts
package main

import (
	"flag"
	"log"

	"github.com/scriptertoufiq/go-mvc/config"
	"github.com/scriptertoufiq/go-mvc/internal/database"
	"github.com/scriptertoufiq/go-mvc/internal/database/seeders"
)

func main() {
	fresh := flag.Bool("fresh", false, "drop all tables before migrating (destructive)")
	seed := flag.Bool("seed", false, "run seeders after migrating")
	rollback := flag.Bool("rollback", false, "reverse the most recent batch of migrations (destructive)")
	steps := flag.Int("steps", 1, "with -rollback: how many batches to reverse")
	status := flag.Bool("status", false, "list every migration and whether it has been applied")
	force := flag.Bool("force", false, "permit -rollback when APP_ENV=production")
	flag.Parse()

	cfg := config.Load()

	// Guard rail: -fresh wipes data, so never let it run against production.
	// There is deliberately no -force escape hatch — rebuilding a production
	// database from empty is not an operation this command should offer.
	if *fresh && cfg.App.IsProduction() {
		log.Fatal("refusing to run -fresh with APP_ENV=production")
	}

	// Rollback drops whatever the last batch created, which in production is
	// real data. It is a legitimate thing to need after a bad deploy, so it is
	// gated rather than forbidden.
	if *rollback && cfg.App.IsProduction() && !*force {
		log.Fatal("refusing to roll back with APP_ENV=production without -force")
	}

	// Seeding is development-only. The fixtures include an admin account whose
	// password is published in the README, so running this anywhere real would
	// hand over the application. Gated on an allow-list of known dev
	// environments so an unfamiliar APP_ENV fails closed.
	if *seed && !cfg.App.IsDevelopment() {
		log.Fatalf(
			"refusing to seed with APP_ENV=%q — seeding creates fixture accounts with known passwords "+
				"and is only permitted when APP_ENV is one of: local, development, dev, test, testing",
			cfg.App.Env)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer func() { _ = database.Close(db) }()

	// A report, not a change — nothing else should run alongside it.
	if *status {
		if err := database.Status(db); err != nil {
			log.Fatalf("reading migration status failed: %v", err)
		}
		return
	}

	switch {
	case *rollback:
		// Rolling back and then seeding would seed a schema that was just
		// partly dismantled, so this path stops here.
		if err := database.Rollback(db, *steps); err != nil {
			log.Fatalf("rollback failed: %v", err)
		}
		return
	case *fresh:
		err = database.Fresh(db)
	default:
		err = database.Migrate(db)
	}
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	if *seed {
		if err := seeders.Run(db); err != nil {
			log.Fatalf("seeding failed: %v", err)
		}
	}
}
