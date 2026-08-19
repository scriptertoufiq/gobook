package reactions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/internal/repositories"
)

// Flusher moves reactions from Redis into MySQL on an interval.
//
// This is the half of write-behind that makes the data real. Everything before
// it is fast because it is not durable; this is where that debt is paid.
type Flusher struct {
	store *Store
	repo  repositories.ReactionRepository
	db    *gorm.DB

	interval time.Duration
	timeout  time.Duration
}

func NewFlusher(
	store *Store,
	repo repositories.ReactionRepository,
	db *gorm.DB,
	interval time.Duration,
) *Flusher {
	if interval <= 0 {
		interval = 10 * time.Second
	}

	return &Flusher{
		store:    store,
		repo:     repo,
		db:       db,
		interval: interval,
		// Generous next to the interval: a slow batch should finish rather
		// than be abandoned and retried, which would only make the queue longer.
		timeout: 30 * time.Second,
	}
}

// Start runs the flusher until stop is closed, then drains once more.
//
// The final drain is what makes an ordinary deploy lossless: without it,
// everything reacted to since the last tick would be discarded with the process.
func (f *Flusher) Start(stop <-chan struct{}) {
	go func() {
		// Anything a previous process claimed and never released is written
		// first, before new work is claimed on top of it.
		f.recover()

		ticker := time.NewTicker(f.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				f.tick()
			case <-stop:
				log.Println("reactions: draining before shutdown")
				f.tick()
				return
			}
		}
	}()
}

// tick claims whatever is pending and writes it.
func (f *Flusher) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	runID, err := newRunID()
	if err != nil {
		log.Printf("reactions: could not name a flush run: %v", err)
		return
	}

	batch, err := f.store.Claim(ctx, runID)
	if err != nil {
		log.Printf("reactions: claiming pending writes failed: %v", err)
		return
	}
	if batch.Len() == 0 {
		return
	}

	if err := f.write(ctx, batch); err != nil {
		// The claimed set is deliberately left in place. It is the only record
		// that these writes are outstanding, and recover() will find it.
		log.Printf("reactions: flush %s failed, %d write(s) held for retry: %v",
			runID, batch.Len(), err)
		return
	}

	log.Printf("reactions: flushed %d upsert(s) and %d removal(s)",
		len(batch.Upserts), len(batch.Deletes))
}

// write persists a batch and only then forgets it.
//
// Both statements share a transaction so a post cannot end up with a reaction
// recorded and its removal lost. The Redis delete cannot join that transaction,
// so it happens after the commit — making the survivable failure the one that
// occurs: dying in between replays a batch that is already written, and both
// statements are idempotent.
func (f *Flusher) write(ctx context.Context, batch Batch) error {
	rows := make([]models.Reaction, 0, len(batch.Upserts))
	for i, pair := range batch.Upserts {
		rows = append(rows, models.Reaction{
			PostID: pair.PostID,
			UserID: pair.UserID,
			Type:   batch.Types[i],
		})
	}

	err := f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := f.repo.WithTx(tx)

		if err := repo.UpsertBatch(ctx, rows); err != nil {
			return err
		}
		return repo.DeleteBatch(ctx, batch.Deletes)
	})
	if err != nil {
		return err
	}

	// The removal markers have served their purpose now that the deletions are
	// durable. Dropping them is what keeps Redis holding only real reactions.
	if err := f.store.ClearRemoved(ctx, batch.Deletes); err != nil {
		log.Printf("reactions: %v", err)
	}

	return f.store.Release(ctx, batch.RunID)
}

// recover writes batches a previous process claimed and never released.
//
// Without this a crash mid-flush would strand those reactions in a set nothing
// ever looks at again — present in Redis, absent from MySQL, and invisible.
func (f *Flusher) recover() {
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	runIDs, err := f.store.Orphans(ctx)
	if err != nil {
		log.Printf("reactions: could not scan for abandoned batches: %v", err)
		return
	}
	if len(runIDs) == 0 {
		return
	}

	log.Printf("reactions: recovering %d abandoned batch(es) from a previous run", len(runIDs))

	for _, runID := range runIDs {
		batch, err := f.store.Resume(ctx, runID)
		if err != nil {
			log.Printf("reactions: could not resume batch %s: %v", runID, err)
			continue
		}

		if batch.Len() == 0 {
			// Nothing left in it — release so it stops being rescanned.
			if err := f.store.Release(ctx, runID); err != nil {
				log.Printf("reactions: could not release empty batch %s: %v", runID, err)
			}
			continue
		}

		if err := f.write(ctx, batch); err != nil {
			log.Printf("reactions: recovering batch %s failed, left for the next start: %v", runID, err)
			continue
		}

		log.Printf("reactions: recovered %d write(s) from batch %s", batch.Len(), runID)
	}
}

// newRunID names a claimed batch. Random rather than sequential so two
// instances starting at the same moment cannot pick the same name.
func newRunID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("reactions: read random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
