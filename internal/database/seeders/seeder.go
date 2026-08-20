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
	{"demo users", SeedDemoUsers},
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

// demoUserCount is how many extra accounts SeedDemoUsers creates.
//
// Enough to make a feed look like a feed and to give reactions and comments
// more than three distinct authors, without making `make migrate-fresh` slow.
const demoUserCount = 100

// firstNames and lastNames are combined by index, so the same run always
// produces the same names — a fixture that changes between runs is a fixture
// you cannot write an assertion against.
var (
	firstNames = []string{
		"Ayesha", "Rafi", "Nusrat", "Tanvir", "Sadia", "Imran", "Farhana", "Shakib",
		"Mitu", "Arif", "Rumana", "Nayeem", "Tasnim", "Sabbir", "Jannat", "Rakib",
		"Sumaiya", "Fahim", "Mahi", "Zahid",
	}
	lastNames = []string{"Rahman", "Ahmed", "Hossain", "Islam", "Chowdhury"}
)

// SeedDemoUsers fills the app with ordinary members.
//
// Every one shares the password below, which is why this — like the rest of the
// seeders — only runs when APP_ENV is a development environment. Accounts are
// pre-verified so they stay usable with AUTH_REQUIRE_EMAIL_VERIFICATION=true;
// nobody can click a link on their behalf.
func SeedDemoUsers(db *gorm.DB) error {
	// One hash for all of them. bcrypt is deliberately slow, and hashing the
	// same password a hundred times would make this the longest part of a
	// fresh migration for no benefit.
	password, err := hash.Make("password")
	if err != nil {
		return err
	}

	now := time.Now()
	users := make([]models.User, 0, demoUserCount)

	for i := 1; i <= demoUserCount; i++ {
		name := fmt.Sprintf("%s %s",
			firstNames[(i-1)%len(firstNames)],
			lastNames[(i-1)/len(firstNames)%len(lastNames)])

		users = append(users, models.User{
			Name:            name,
			Email:           fmt.Sprintf("user%d@example.com", i),
			Password:        password,
			Role:            models.RoleUser,
			IsActive:        true,
			EmailVerifiedAt: &now,
		})
	}

	// Keyed on the unique email index, so running this twice adds nothing.
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoNothing: true}).
		CreateInBatches(&users, 50).Error
}
