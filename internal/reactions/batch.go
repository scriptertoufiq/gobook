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

// claimScript moves at most ARGV[1] pending entries into a claimed batch.
//
// Bounded, and that is the whole point. Taking everything pending is fine while
// the flusher keeps up and catastrophic when it does not: after an outage the
// pending set holds however much accumulated, and one claim would then try to
// hold all of it in memory, pipeline an HGET for every entry, and write it in a
// single transaction that cannot finish inside the timeout — so it rolls back
// and the next tick attempts exactly the same thing. A backlog would never
// drain. A capped claim turns that into several ordinary flushes.
//
// SPOP and SADD together, so the entries are never in neither set: they are
// claimed atomically and stay claimed until the database write commits.
var claimScript = redis.NewScript(`
local members = redis.call('SPOP', KEYS[1], tonumber(ARGV[1]))
if #members == 0 then
  return 0
end

-- unpack has a stack limit, so hand them over in slices.
for i = 1, #members, 500 do
  local last = math.min(i + 499, #members)
  redis.call('SADD', KEYS[2], unpack(members, i, last))
end

return #members
`)

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

	// Full reports that the claim hit its limit, so more is probably pending.
	Full bool
}

// DefaultClaimLimit caps how many pending writes one flush takes on.
//
// Small enough that the batch fits comfortably in memory, in one pipeline and
// in one transaction; large enough that a busy app still moves thousands of
// reactions a second across a few cycles.
const DefaultClaimLimit = 2000

func (b Batch) Len() int { return len(b.Upserts) + len(b.Deletes) }

// Claim moves up to limit pending entries into a batch and resolves each to
// its current value.
//
// Returns an empty batch and no error when there is nothing pending. The
// entries stay in the claimed set until Release, so a crash before the database
// write loses nothing — the orphan scan finds them at the next start.
func (s *Store) Claim(ctx context.Context, runID string, limit int) (Batch, error) {
	if limit <= 0 {
		limit = DefaultClaimLimit
	}

	claimed, err := claimScript.Run(ctx, s.client,
		[]string{
			s.key(cachekeys.ReactionDirty()),
			s.key(cachekeys.ReactionFlushing(runID)),
		},
		limit).Int64()
	if err != nil {
		return Batch{}, fmt.Errorf("reactions: claim pending writes: %w", err)
	}
	if claimed == 0 {
		return Batch{}, nil
	}

	batch, err := s.resolve(ctx, runID)
	if err != nil {
		return Batch{}, err
	}

	// Full means there is very likely more waiting, which the caller uses to
	// keep draining instead of sleeping until the next tick.
	batch.Full = claimed >= int64(limit)
	return batch, nil
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
