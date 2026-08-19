package reactions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/scriptertoufiq/gobook/internal/cachekeys"
	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/internal/repositories"
)

// scanBatch is how many pending entries are read per SSCAN round trip.
const scanBatch = 512

// parsePair reverses the "postID:userID" token used in the pending set.
func parsePair(token string) (repositories.PostUser, error) {
	postRaw, userRaw, ok := strings.Cut(token, ":")
	if !ok {
		return repositories.PostUser{}, fmt.Errorf("reactions: malformed pending entry %q", token)
	}

	postID, err := strconv.ParseUint(postRaw, 10, 64)
	if err != nil {
		return repositories.PostUser{}, fmt.Errorf("reactions: malformed post id in %q", token)
	}
	userID, err := strconv.ParseUint(userRaw, 10, 64)
	if err != nil {
		return repositories.PostUser{}, fmt.Errorf("reactions: malformed user id in %q", token)
	}

	return repositories.PostUser{PostID: uint(postID), UserID: uint(userID)}, nil
}

// Batch is a set of pending writes claimed for one flush.
type Batch struct {
	// RunID names the claimed set so it can be released, or found again after
	// a crash.
	RunID string

	// Upserts are people who currently have a reaction.
	Upserts []repositories.PostUser
	// Types is the reaction each upsert pair resolves to, index-aligned.
	Types []string

	// Deletes are people who took theirs back.
	Deletes []repositories.PostUser
}

func (b Batch) Len() int { return len(b.Upserts) + len(b.Deletes) }

// Claim moves the pending set aside and resolves every pair to its current value.
//
// RENAME rather than SPOP, and it is the difference between a recoverable
// flush and a lossy one. Renaming hands the batch over atomically — new
// reactions immediately start filling a fresh set — while leaving the claimed
// one intact, so a crash before the database write loses nothing. Popping
// would take the entries out of Redis before MySQL had them.
//
// Returns an empty batch and no error when there is nothing pending.
func (s *Store) Claim(ctx context.Context, runID string) (Batch, error) {
	from := s.key(cachekeys.ReactionDirty())
	to := s.key(cachekeys.ReactionFlushing(runID))

	if err := s.client.Rename(ctx, from, to).Err(); err != nil {
		// An empty queue is the common case, and RENAME reports it as a plain
		// "ERR no such key" rather than redis.Nil — so it has to be matched on
		// the message. Treating it as a fault would log an error every idle tick.
		if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "no such key") {
			return Batch{}, nil
		}
		return Batch{}, fmt.Errorf("reactions: claim pending set: %w", err)
	}

	return s.resolve(ctx, runID)
}

// resolve reads the current value of every pair in a claimed set and sorts
// them into upserts and deletes.
func (s *Store) resolve(ctx context.Context, runID string) (Batch, error) {
	setKey := s.key(cachekeys.ReactionFlushing(runID))

	// SSCAN rather than SMEMBERS. If the flusher ever falls behind — a traffic
	// spike, a slow database — the claimed set can hold hundreds of thousands
	// of entries, and SMEMBERS would pull all of them back in one blocking
	// call. Redis is single-threaded, so that stalls every other client at
	// exactly the moment the app is already struggling.
	var (
		cursor uint64
		tokens []string
	)
	for {
		page, next, err := s.client.SScan(ctx, setKey, cursor, "", scanBatch).Result()
		if err != nil {
			return Batch{}, fmt.Errorf("reactions: read claimed set %s: %w", runID, err)
		}

		tokens = append(tokens, page...)
		cursor = next
		if cursor == 0 {
			break
		}
	}

	if len(tokens) == 0 {
		return Batch{RunID: runID}, nil
	}

	// One round trip for the lot, rather than one per pair.
	pipe := s.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(tokens))
	pairs := make([]repositories.PostUser, 0, len(tokens))
	valid := make([]int, 0, len(tokens))

	for i, token := range tokens {
		pair, err := parsePair(token)
		if err != nil {
			// A malformed entry cannot be acted on and would jam every future
			// flush if it were left in place. Skipping drops it with the batch.
			continue
		}
		cmds[i] = pipe.HGet(ctx, s.key(cachekeys.ReactionByUser(pair.UserID)), field(pair.PostID))
		pairs = append(pairs, pair)
		valid = append(valid, i)
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return Batch{}, fmt.Errorf("reactions: resolve claimed set %s: %w", runID, err)
	}

	batch := Batch{RunID: runID}

	for n, idx := range valid {
		pair := pairs[n]

		raw, err := cmds[idx].Result()
		if errors.Is(err, redis.Nil) {
			// Absent is skipped, never treated as a removal.
			//
			// A removal always writes the '-' sentinel, so absence can only mean
			// the value went missing — expired, evicted, or flushed by another
			// instance between the claim and this read. Deleting on that basis
			// would erase a reaction the person still holds, and the moment
			// these keys carry a TTL that stops being hypothetical. Leaving the
			// row alone is the recoverable choice: the worst case is a stored
			// reaction that is briefly out of date, which the next write or the
			// reconcile pass corrects.
			continue
		}
		if err != nil {
			return Batch{}, fmt.Errorf("reactions: read pending value: %w", err)
		}

		// The stored value is "type|millis" — the timestamp must be stripped
		// before anything reaches the type column.
		reaction, _ := decodeValue(raw)

		if reaction == None {
			batch.Deletes = append(batch.Deletes, pair)
			continue
		}

		// Anything that is not a reaction this app recognises is dropped rather
		// than written. A malformed value can only come from a bug, and the
		// database is the wrong place to discover one.
		if !models.IsValidReaction(reaction) {
			log.Printf("reactions: discarding unrecognised value %q for post %d user %d",
				raw, pair.PostID, pair.UserID)
			continue
		}

		batch.Upserts = append(batch.Upserts, pair)
		batch.Types = append(batch.Types, reaction)
	}

	return batch, nil
}

// ClearRemoved deletes the fields of reactions that have just been persisted
// as removals.
//
// The '-' marker exists only so the flusher can tell "took it back" from "never
// reacted". Once the deletion is in the database it has done its job, and
// leaving it would put the app back to holding a record per person per post —
// the very thing keying by user avoids. Called after the commit, so a failure
// here costs a stale marker, never a lost write.
func (s *Store) ClearRemoved(ctx context.Context, pairs []repositories.PostUser) error {
	if len(pairs) == 0 {
		return nil
	}

	byUser := make(map[uint][]string, len(pairs))
	for _, p := range pairs {
		byUser[p.UserID] = append(byUser[p.UserID], field(p.PostID))
	}

	pipe := s.client.Pipeline()
	for userID, fields := range byUser {
		pipe.HDel(ctx, s.key(cachekeys.ReactionByUser(userID)), fields...)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("reactions: clear %d persisted removal(s): %w", len(pairs), err)
	}
	return nil
}

// Release drops a claimed set. Called only after the database write commits —
// until then the set is the only record that these writes are outstanding.
func (s *Store) Release(ctx context.Context, runID string) error {
	if err := s.client.Del(ctx, s.key(cachekeys.ReactionFlushing(runID))).Err(); err != nil {
		return fmt.Errorf("reactions: release claimed set %s: %w", runID, err)
	}
	return nil
}

// Orphans finds batches a previous process claimed and never released, so a
// crash mid-flush costs nothing once the app comes back.
func (s *Store) Orphans(ctx context.Context) ([]string, error) {
	pattern := s.key(cachekeys.ReactionFlushingPrefix()) + ":*"
	prefix := s.key(cachekeys.ReactionFlushingPrefix()) + ":"

	var (
		cursor uint64
		runIDs []string
	)

	for {
		keys, next, err := s.client.Scan(ctx, cursor, pattern, 64).Result()
		if err != nil {
			return nil, fmt.Errorf("reactions: scan for abandoned batches: %w", err)
		}

		for _, k := range keys {
			runIDs = append(runIDs, strings.TrimPrefix(k, prefix))
		}

		cursor = next
		if cursor == 0 {
			return runIDs, nil
		}
	}
}

// Resume re-reads an abandoned batch so it can be written and released.
func (s *Store) Resume(ctx context.Context, runID string) (Batch, error) {
	return s.resolve(ctx, runID)
}

// Pending reports how many writes are waiting. Worth a metric: if this climbs
// steadily the flusher is not keeping up with the write rate.
func (s *Store) Pending(ctx context.Context) (int64, error) {
	n, err := s.client.SCard(ctx, s.key(cachekeys.ReactionDirty())).Result()
	if err != nil {
		return 0, fmt.Errorf("reactions: count pending writes: %w", err)
	}
	return n, nil
}
