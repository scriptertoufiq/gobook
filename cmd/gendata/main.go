// Command gendata fills the database with development data at scale.
//
//	go run ./cmd/gendata -posts 2000000            # spread across every user
//	go run ./cmd/gendata -posts 50000 -workers 8
//	go run ./cmd/gendata -clear                    # remove what it generated
//
// This is not a seeder. Seeders describe the fixtures an app needs to run;
// this exists to answer questions about behaviour at volume, and is refused
// outside a development environment for the same reason seeding is.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/config"
	"github.com/scriptertoufiq/gobook/internal/database"
)

// generatedMarker tags every row this command writes, so -clear can remove
// exactly what it made and nothing a person actually wrote.
const generatedMarker = "​" // zero-width space, invisible in any UI

var topics = []string{
	"shipping on a Friday", "the migration that would not roll back", "why the cache lied",
	"reading the query plan", "a bug that only happened at midnight", "deleting code",
	"the interview question nobody answers well", "what the logs did not say",
	"three ways to break a foreign key", "when the retry made it worse",
	"naming things, again", "the outage nobody noticed", "a faster path that was slower",
	"tests that pass for the wrong reason", "the index we forgot",
}

var openers = []string{
	"Spent the morning on this and it turned out to be simpler than expected.",
	"Took me far too long to work out what was actually happening here.",
	"Writing this down mostly so I remember it next time.",
	"A small thing, but it has bitten me twice now.",
	"Not sure this is the right approach, but it works and I can explain why.",
	"The obvious answer was wrong, which is the interesting part.",
}

var closers = []string{
	"Curious whether anyone has hit the same thing.",
	"Happy to be told there is a better way.",
	"Will follow up once it has been running for a week.",
	"The fix was one line; finding it was not.",
	"Leaving the details here in case it saves somebody an afternoon.",
}

func main() {
	posts := flag.Int("posts", 0, "how many posts to generate")
	batch := flag.Int("batch", 1000, "rows per INSERT statement")
	workers := flag.Int("workers", 4, "concurrent writers")
	clear := flag.Bool("clear", false, "remove every generated post instead of adding more")
	flag.Parse()

	cfg := config.Load()

	// Same guard as seeding: this writes volumes of fake content, and an
	// unfamiliar APP_ENV should fail closed rather than fill a real database.
	if !cfg.App.IsDevelopment() {
		log.Fatalf("refusing to generate data with APP_ENV=%q — development environments only", cfg.App.Env)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer func() { _ = database.Close(db) }()

	if *clear {
		clearGenerated(db)
		return
	}

	if *posts <= 0 {
		log.Fatal("nothing to do — pass -posts N")
	}

	var userIDs []uint
	if err := db.Raw("SELECT id FROM users WHERE deleted_at IS NULL").Scan(&userIDs).Error; err != nil {
		log.Fatalf("reading users: %v", err)
	}
	if len(userIDs) == 0 {
		log.Fatal("no users to attribute posts to — run `make seed` first")
	}

	log.Printf("generating %d post(s) across %d user(s), %d workers, %d rows per statement",
		*posts, len(userIDs), *workers, *batch)

	start := time.Now()
	var written int64

	// Progress on its own timer rather than per batch, so reporting never
	// becomes the slow part.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n := atomic.LoadInt64(&written)
				elapsed := time.Since(start).Seconds()
				log.Printf("  %d/%d (%.0f%%) at %.0f rows/s",
					n, *posts, float64(n)/float64(*posts)*100, float64(n)/elapsed)
			case <-done:
				return
			}
		}
	}()

	var wg sync.WaitGroup
	share := *posts / *workers

	for w := 0; w < *workers; w++ {
		count := share
		if w == *workers-1 {
			count = *posts - share*(*workers-1) // the last one takes the remainder
		}

		wg.Add(1)
		go func(seed int64, count int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))

			for remaining := count; remaining > 0; {
				size := min(*batch, remaining)
				if err := insertBatch(db, rng, userIDs, size); err != nil {
					log.Printf("batch failed: %v", err)
					return
				}
				remaining -= size
				atomic.AddInt64(&written, int64(size))
			}
		}(int64(w)+1, count)
	}

	wg.Wait()
	close(done)

	elapsed := time.Since(start)
	log.Printf("done: %d posts in %s (%.0f rows/s)",
		atomic.LoadInt64(&written), elapsed.Round(time.Millisecond),
		float64(atomic.LoadInt64(&written))/elapsed.Seconds())
}

// insertBatch writes one multi-row INSERT.
//
// Raw SQL rather than the repository: this is bulk loading, and going through
// the model layer would pay reflection and hook costs a million times over for
// rows nobody will read individually.
func insertBatch(db *gorm.DB, rng *rand.Rand, userIDs []uint, size int) error {
	var sql strings.Builder
	sql.WriteString("INSERT INTO posts (user_id, title, content, created_at, updated_at) VALUES ")

	args := make([]any, 0, size*3)

	for i := range size {
		if i > 0 {
			sql.WriteString(",")
		}
		sql.WriteString("(?,?,?,NOW(3),NOW(3))")

		title := fmt.Sprintf("On %s", topics[rng.Intn(len(topics))])
		content := fmt.Sprintf("%s %s %s",
			openers[rng.Intn(len(openers))],
			topics[rng.Intn(len(topics))],
			closers[rng.Intn(len(closers))])

		args = append(args,
			userIDs[rng.Intn(len(userIDs))],
			title+generatedMarker,
			content)
	}

	return db.Exec(sql.String(), args...).Error
}

func clearGenerated(db *gorm.DB) {
	log.Println("removing generated posts (and, through the foreign keys, their comments and reactions)")

	start := time.Now()
	var removed int64

	// Deleted in chunks so one enormous transaction never holds the table.
	for {
		res := db.Exec("DELETE FROM posts WHERE title LIKE ? LIMIT 5000", "%"+generatedMarker+"%")
		if res.Error != nil {
			log.Fatalf("delete failed: %v", res.Error)
		}
		removed += res.RowsAffected
		if res.RowsAffected == 0 {
			break
		}
		log.Printf("  removed %d so far", removed)
	}

	log.Printf("done: %d generated post(s) removed in %s", removed, time.Since(start).Round(time.Millisecond))
}
