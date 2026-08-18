package migrations

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Record is one row of the ledger: proof that a migration has already run.
//
// It deliberately does not embed models.Base. A soft-deletable ledger row is a
// contradiction — GORM's default scope would hide it, the migration would look
// pending, and it would run a second time.
type Record struct {
	ID        uint      `gorm:"primaryKey"`
	Migration string    `gorm:"size:191;not null;uniqueIndex"`
	Batch     int       `gorm:"not null;index"`
	AppliedAt time.Time `gorm:"not null"`
}

func (Record) TableName() string { return "migrations" }

// Prepare creates the ledger table.
//
// This is not itself a migration, and cannot be: the table that records what
// has run has to exist before anything can be recorded in it.
func Prepare(db *gorm.DB) error {
	if err := db.AutoMigrate(&Record{}); err != nil {
		return fmt.Errorf("migrations: create ledger table: %w", err)
	}
	return nil
}

// appliedIDs returns the set of migrations the ledger says have already run.
func appliedIDs(db *gorm.DB) (map[string]bool, error) {
	var ids []string
	if err := db.Model(&Record{}).Pluck("migration", &ids).Error; err != nil {
		return nil, fmt.Errorf("migrations: read ledger: %w", err)
	}

	applied := make(map[string]bool, len(ids))
	for _, id := range ids {
		applied[id] = true
	}
	return applied, nil
}

// Pending returns the registered migrations that have not run yet, in order.
func Pending(db *gorm.DB) ([]Migration, error) {
	applied, err := appliedIDs(db)
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, m := range All() {
		if !applied[m.ID] {
			pending = append(pending, m)
		}
	}
	return pending, nil
}

// nextBatch is the batch number this run will be recorded under. Everything
// applied together shares one, so a rollback undoes a deploy rather than an
// arbitrary number of individual changes.
func nextBatch(db *gorm.DB) (int, error) {
	var highest *int
	if err := db.Model(&Record{}).Select("MAX(batch)").Scan(&highest).Error; err != nil {
		return 0, fmt.Errorf("migrations: read current batch: %w", err)
	}

	if highest == nil {
		return 1, nil
	}
	return *highest + 1, nil
}

// Up applies every pending migration in ID order and returns the ones that ran.
//
// There is no transaction around the loop, and that is a deliberate choice
// rather than an omission: MySQL commits DDL implicitly, so a rollback after a
// failed CREATE TABLE would restore nothing while appearing to. Instead each
// migration is recorded the instant it succeeds, which leaves the ledger
// accurate up to the point of failure — re-running resumes at the one that
// broke instead of repeating the ones that worked.
func Up(db *gorm.DB) ([]string, error) {
	if err := Prepare(db); err != nil {
		return nil, err
	}

	pending, err := Pending(db)
	if err != nil {
		return nil, err
	}
	if len(pending) == 0 {
		return nil, nil
	}

	batch, err := nextBatch(db)
	if err != nil {
		return nil, err
	}

	applied := make([]string, 0, len(pending))
	for _, m := range pending {
		log.Printf("migrate: running %s", m.ID)

		if err := m.Up(db); err != nil {
			return applied, fmt.Errorf("migrations: %s failed: %w", m.ID, err)
		}

		record := &Record{Migration: m.ID, Batch: batch, AppliedAt: time.Now()}
		if err := db.Create(record).Error; err != nil {
			// The schema changed but the ledger did not, so a re-run would
			// apply it twice. Say so plainly — this needs a human.
			return applied, fmt.Errorf(
				"migrations: %s ran but could not be recorded, so the ledger is now behind the schema "+
					"(insert it by hand before re-running): %w", m.ID, err)
		}

		applied = append(applied, m.ID)
	}

	return applied, nil
}

// Rollback reverses the last `batches` batches, newest migration first, and
// returns the IDs it undid.
func Rollback(db *gorm.DB, batches int) ([]string, error) {
	if batches < 1 {
		batches = 1
	}
	if err := Prepare(db); err != nil {
		return nil, err
	}

	var targets []int
	err := db.Model(&Record{}).
		Select("batch").
		Group("batch").
		Order("batch desc").
		Limit(batches).
		Pluck("batch", &targets).Error
	if err != nil {
		return nil, fmt.Errorf("migrations: read batches: %w", err)
	}
	if len(targets) == 0 {
		return nil, nil
	}

	var records []Record
	// Reverse of the order they were applied in: a table created last is the
	// one holding the foreign keys, so it has to go first.
	if err := db.Where("batch IN ?", targets).Order("migration desc").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("migrations: read ledger: %w", err)
	}

	// Check the whole set before touching any of it. Discovering half way
	// through that the next one cannot be reversed would leave the schema in a
	// state no version of the code describes.
	for _, r := range records {
		m, ok := find(r.Migration)
		switch {
		case !ok:
			return nil, fmt.Errorf(
				"migrations: %s is in the ledger but no longer registered — restore the file to roll it back",
				r.Migration)
		case m.Down == nil:
			return nil, fmt.Errorf(
				"migrations: %s is irreversible (no Down), so this batch cannot be rolled back",
				r.Migration)
		}
	}

	rolled := make([]string, 0, len(records))
	for _, r := range records {
		m, _ := find(r.Migration)
		log.Printf("migrate: rolling back %s", r.Migration)

		if err := m.Down(db); err != nil {
			return rolled, fmt.Errorf("migrations: rolling back %s failed: %w", r.Migration, err)
		}

		if err := db.Delete(&Record{}, r.ID).Error; err != nil {
			return rolled, fmt.Errorf(
				"migrations: %s was reversed but its ledger row survived, so it now looks applied: %w",
				r.Migration, err)
		}

		rolled = append(rolled, r.Migration)
	}

	return rolled, nil
}

// State is one line of the status report.
type State struct {
	ID        string
	Applied   bool
	Batch     int
	AppliedAt time.Time

	// Orphaned marks a ledger row whose file is gone. It is reported rather
	// than ignored: the database carries a change nothing in the code
	// describes, and rollback cannot reverse it.
	Orphaned bool
}

// Status reports every migration — registered, applied, or orphaned.
func Status(db *gorm.DB) ([]State, error) {
	if err := Prepare(db); err != nil {
		return nil, err
	}

	var records []Record
	if err := db.Order("migration asc").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("migrations: read ledger: %w", err)
	}

	byID := make(map[string]Record, len(records))
	for _, r := range records {
		byID[r.Migration] = r
	}

	states := make([]State, 0, len(records))
	for _, m := range All() {
		state := State{ID: m.ID}
		if record, ok := byID[m.ID]; ok {
			state.Applied = true
			state.Batch = record.Batch
			state.AppliedAt = record.AppliedAt
		}
		states = append(states, state)
	}

	for _, r := range records {
		if _, ok := find(r.Migration); !ok {
			states = append(states, State{
				ID: r.Migration, Applied: true, Batch: r.Batch,
				AppliedAt: r.AppliedAt, Orphaned: true,
			})
		}
	}

	return states, nil
}

// Fresh drops every table in the database and replays the full history.
//
// Every table, not merely the ones the migrations know about: a table left
// behind by a since-deleted migration would otherwise survive a "fresh"
// database and quietly keep it different from a colleague's.
func Fresh(db *gorm.DB) ([]string, error) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return nil, fmt.Errorf("migrations: list tables: %w", err)
	}

	if len(tables) > 0 {
		log.Printf("migrate: dropping %d table(s)", len(tables))

		drop := make([]any, len(tables))
		for i, name := range tables {
			drop[i] = name
		}

		// The MySQL driver's DropTable turns foreign key checks off for the
		// duration, on a single connection, so drop order does not matter here.
		if err := db.Migrator().DropTable(drop...); err != nil {
			return nil, fmt.Errorf("migrations: drop tables: %w", err)
		}
	}

	return Up(db)
}
