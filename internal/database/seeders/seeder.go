// Package seeders fills the database with baseline/demo rows.
package seeders

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/pkg/hash"
)

// seeders runs in slice order, so a seeder may depend on the ones above it.
var seeders = []struct {
	name string
	run  func(*gorm.DB) error
}{
	{"users", SeedUsers},
}

// Run executes every seeder in order. Seeders are idempotent — running them
// twice must not duplicate rows.
func Run(db *gorm.DB) error {
	for _, s := range seeders {
		log.Printf("seed: %s", s.name)
		if err := s.run(db); err != nil {
			return fmt.Errorf("seed %s: %w", s.name, err)
		}
	}

	log.Println("seed: done")
	return nil
}

func SeedUsers(db *gorm.DB) error {
	password, err := hash.Make("password")
	if err != nil {
		return err
	}

	// Seeded accounts are pre-verified so the demo data stays usable when
	// AUTH_REQUIRE_EMAIL_VERIFICATION=true — nobody can click a link for them.
	now := time.Now()

	users := []models.User{
		{
			Name: "Admin", Email: "admin@example.com", Password: password,
			Role: models.RoleAdmin, IsActive: true, EmailVerifiedAt: &now,
		},
		{
			Name: "Jane Editor", Email: "jane@example.com", Password: password,
			Role: models.RoleUser, IsActive: true, EmailVerifiedAt: &now,
		},
	}

	// ON CONFLICT DO NOTHING keyed on the unique email index.
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoNothing: true}).
		Create(&users).Error
}
