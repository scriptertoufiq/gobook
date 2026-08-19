// Package reactions is the hot tier: Redis holds the authoritative state of
// every reaction and answers every read, while a background flusher folds
// changes into MySQL in batches.
//
// It keeps its own Redis client rather than going through pkg/cache. That
// package is a JSON get/set/delete cache by design; reactions need hashes,
// sets and Lua, and widening the general cache to fit one feature would make
// it worse at the job it already does.
//
// # How memory is bounded
//
// A person's reactions live in one hash keyed by user, holding a field only
// for posts they actually reacted to. The obvious alternative — a key per
// (post, user) — grows with *views* rather than with reactions, because every
// reader of every post leaves a permanent trace behind. Measured on this
// setup that is 103 bytes per view against 29 bytes per reaction: 19 GB versus
// 1.4 GB once the app is large. Nothing is written for somebody who merely
// reads a post.
package reactions

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/scriptertoufiq/gobook/internal/cachekeys"
)

// None marks a reaction that was taken back but not yet written to the
// database.
//
// It is a short-lived marker, not storage: the flusher deletes the field once
// the removal has been persisted. Keeping it any longer would put the app back
// to holding a record per person per post, which is exactly what this layout
// avoids. A person who never reacted has no field at all.
const None = "-"

// A stored reaction is "type|unixMillis" — the choice, and when it was made.
//
// The timestamp is what lets a reaction queued on a phone with no signal be
// replayed later without stamping on a newer one made since.
const valueSeparator = "|"

func encodeValue(reaction string, atMillis int64) string {
	return reaction + valueSeparator + strconv.FormatInt(atMillis, 10)
}

// decodeValue splits a stored value. A value with no separator is read as a
// bare reaction at time zero, so anything newer replaces it.
func decodeValue(raw string) (string, int64) {
	reaction, at, ok := strings.Cut(raw, valueSeparator)
	if !ok {
		return raw, 0
	}

	millis, err := strconv.ParseInt(at, 10, 64)
	if err != nil {
		return reaction, 0
	}
	return reaction, millis
}

func field(postID uint) string { return strconv.FormatUint(uint64(postID), 10) }

// Options configures the store's own connection pool.
type Options struct {
	Addr     string
	Username string
	Password string
	DB       int
	Prefix   string

	DialTimeout time.Duration
	Timeout     time.Duration
	PoolSize    int
}

type Store struct {
	client  *redis.Client
	prefix  string
	set     *redis.Script
	hydrate *redis.Script
}

// setScript applies a reaction and keeps the tally in step, indivisibly.
//
// This has to be a script. Changing love to angry means decrementing one field
// and incrementing another; sent as two commands another request can interleave
// between them, and the counts drift away from the reactions they count —
// permanently, because nothing on the read path recomputes them.
//
//	KEYS[1] the person's reaction hash   KEYS[2] the post's tally   KEYS[3] pending set
//	ARGV[1] new reaction                 ARGV[2] "postID:userID"
//	ARGV[3] action time in millis        ARGV[4] the post id, as a hash field
var setScript = redis.NewScript(`
local raw = redis.call('HGET', KEYS[1], ARGV[4])
local old, oldAt = '-', 0

if raw then
  local sep = string.find(raw, '|', 1, true)
  if sep then
    old   = string.sub(raw, 1, sep - 1)
    oldAt = tonumber(string.sub(raw, sep + 1)) or 0
  else
    old = raw
  end
end

local new   = ARGV[1]
local newAt = tonumber(ARGV[3])

-- A replayed action that predates what is already stored is discarded. This is
-- the offline queue arriving after the account reacted somewhere else.
if newAt < oldAt then
  return {'stale', old, redis.call('HGETALL', KEYS[2])}
end

-- tapping the same reaction again takes it back
if old == new then new = '-' end

if old ~= '-' then
  redis.call('HINCRBY', KEYS[2], old, -1)
end
if new ~= '-' then
  redis.call('HINCRBY', KEYS[2], new, 1)
end

redis.call('HSET', KEYS[1], ARGV[4], new .. '|' .. ARGV[3])
redis.call('SADD', KEYS[3], ARGV[2])

return {'applied', new, redis.call('HGETALL', KEYS[2])}
`)

// hydrateScript writes a post's baseline from the database and marks it
// loaded, but only if nobody has done so already.
//
// Atomic for the same reason: between checking the marker and writing the
// baseline, a live reaction could increment a field, and a plain HSET would
// overwrite it back down.
var hydrateScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then
  return 0
end

for i = 1, #ARGV, 2 do
  redis.call('HSET', KEYS[1], ARGV[i], ARGV[i + 1])
end

redis.call('SET', KEYS[2], '1')
return 1
`)

// New opens the pool and verifies it, so a bad address fails at boot rather
// than on the first reaction.
func New(opts Options) (*Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Username:     opts.Username,
		Password:     opts.Password,
		DB:           opts.DB,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.Timeout,
		WriteTimeout: opts.Timeout,
		PoolSize:     opts.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("reactions: ping redis at %s: %w", opts.Addr, err)
	}

	return &Store{client: client, prefix: opts.Prefix, set: setScript, hydrate: hydrateScript}, nil
}

func (s *Store) key(k string) string {
	if s.prefix == "" {
		return k
	}
	return s.prefix + ":" + k
}

func (s *Store) Close() error { return s.client.Close() }

// Tally is a post's reactions as the API reports them.
type Tally struct {
	Counts map[string]int64
	Total  int64
}

// Set applies a reaction made at actedAt.
//
// applied is false when the action was older than what is already stored — a
// queued offline reaction arriving after a newer one. Nothing changes then.
func (s *Store) Set(
	ctx context.Context,
	postID, userID uint,
	reaction string,
	actedAt time.Time,
) (mine string, tally Tally, applied bool, err error) {
	keys := []string{
		s.key(cachekeys.ReactionByUser(userID)),
		s.key(cachekeys.ReactionCounts(postID)),
		s.key(cachekeys.ReactionDirty()),
	}

	raw, err := s.set.Run(ctx, s.client, keys,
		reaction, pairToken(postID, userID), actedAt.UnixMilli(), field(postID)).Slice()
	if err != nil {
		return "", Tally{}, false, fmt.Errorf(
			"reactions: apply %q for user %d on post %d: %w", reaction, userID, postID, err)
	}
	if len(raw) != 3 {
		return "", Tally{}, false, fmt.Errorf("reactions: unexpected script result of length %d", len(raw))
	}

	outcome, _ := raw[0].(string)
	mine, _ = raw[1].(string)

	flat, ok := raw[2].([]any)
	if !ok {
		return "", Tally{}, false, errors.New("reactions: script returned a tally in an unexpected shape")
	}

	return mine, tallyFromFlat(flat), outcome == "applied", nil
}

// Counts reads a post's tally. The bool reports whether the post has been
// hydrated from MySQL — false means the caller has to do that first, because
// an empty hash and an unloaded post look the same.
func (s *Store) Counts(ctx context.Context, postID uint) (Tally, bool, error) {
	pipe := s.client.Pipeline()
	countsCmd := pipe.HGetAll(ctx, s.key(cachekeys.ReactionCounts(postID)))
	hydratedCmd := pipe.Exists(ctx, s.key(cachekeys.ReactionHydrated(postID)))

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return Tally{}, false, fmt.Errorf("reactions: read counts for post %d: %w", postID, err)
	}

	return tallyFromMap(countsCmd.Val()), hydratedCmd.Val() == 1, nil
}

// CountsMany reads tallies for a page of posts in one round trip.
//
// The whole point of the batch: a feed of ten posts costs one pipeline rather
// than ten sequential exchanges. Posts still needing hydration are reported so
// the caller can load them together instead of one query at a time.
func (s *Store) CountsMany(ctx context.Context, postIDs []uint) (map[uint]Tally, []uint, error) {
	if len(postIDs) == 0 {
		return map[uint]Tally{}, nil, nil
	}

	pipe := s.client.Pipeline()
	counts := make([]*redis.MapStringStringCmd, len(postIDs))
	hydrated := make([]*redis.IntCmd, len(postIDs))

	for i, id := range postIDs {
		counts[i] = pipe.HGetAll(ctx, s.key(cachekeys.ReactionCounts(id)))
		hydrated[i] = pipe.Exists(ctx, s.key(cachekeys.ReactionHydrated(id)))
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, nil, fmt.Errorf("reactions: read counts for %d post(s): %w", len(postIDs), err)
	}

	tallies := make(map[uint]Tally, len(postIDs))
	var cold []uint

	for i, id := range postIDs {
		tallies[id] = tallyFromMap(counts[i].Val())
		if hydrated[i].Val() != 1 {
			cold = append(cold, id)
		}
	}

	return tallies, cold, nil
}

// Mine reads one person's choice on one post. The bool reports whether Redis
// could answer — false means this person's reactions are not loaded and MySQL
// still holds the truth.
func (s *Store) Mine(ctx context.Context, postID, userID uint) (string, bool, error) {
	mine, known, err := s.MineMany(ctx, []uint{postID}, userID)
	if err != nil || !known {
		return "", known, err
	}
	return mine[postID], true, nil
}

// UserLoaded reports whether this person's complete set of reactions is in
// Redis.
//
// A method of its own rather than a call to MineMany with no posts: that
// returns early on an empty list, so using it as a probe silently answers
// "loaded" for everybody and nothing is ever warmed.
func (s *Store) UserLoaded(ctx context.Context, userID uint) (bool, error) {
	if userID == 0 {
		return true, nil
	}

	n, err := s.client.Exists(ctx, s.key(cachekeys.ReactionUserLoaded(userID))).Result()
	if err != nil {
		return false, fmt.Errorf("reactions: check whether user %d is loaded: %w", userID, err)
	}
	return n == 1, nil
}

// MineMany reads a person's choices across a page of posts.
//
// One HMGET for the whole page — the reason this layout is keyed by user. A
// missing field means they did not react, which is only a safe conclusion
// because the loaded marker says their full set is present.
func (s *Store) MineMany(ctx context.Context, postIDs []uint, userID uint) (map[uint]string, bool, error) {
	if userID == 0 || len(postIDs) == 0 {
		return map[uint]string{}, true, nil
	}

	pipe := s.client.Pipeline()
	loadedCmd := pipe.Exists(ctx, s.key(cachekeys.ReactionUserLoaded(userID)))

	fields := make([]string, len(postIDs))
	for i, id := range postIDs {
		fields[i] = field(id)
	}
	valuesCmd := pipe.HMGet(ctx, s.key(cachekeys.ReactionByUser(userID)), fields...)

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, false, fmt.Errorf("reactions: read reactions of user %d: %w", userID, err)
	}

	if loadedCmd.Val() != 1 {
		return nil, false, nil
	}

	out := make(map[uint]string, len(postIDs))
	for i, raw := range valuesCmd.Val() {
		text, ok := raw.(string)
		if !ok {
			continue // absent field: they did not react to this one
		}

		reaction, _ := decodeValue(text)
		if reaction != None {
			out[postIDs[i]] = reaction
		}
	}

	return out, true, nil
}

// Hydrate seeds a post's tally from the database and marks it loaded, unless
// that has already happened.
func (s *Store) Hydrate(ctx context.Context, postID uint, counts map[string]int64) error {
	keys := []string{
		s.key(cachekeys.ReactionCounts(postID)),
		s.key(cachekeys.ReactionHydrated(postID)),
	}

	args := make([]any, 0, len(counts)*2)
	for reaction, n := range counts {
		args = append(args, reaction, n)
	}

	if err := s.hydrate.Run(ctx, s.client, keys, args...).Err(); err != nil {
		return fmt.Errorf("reactions: hydrate post %d: %w", postID, err)
	}
	return nil
}

// LoadUser writes a person's complete set of stored reactions and marks them
// loaded.
//
// Done once per person rather than once per person per post — the trade that
// keeps nothing on record for the posts they only read. A user with no
// reactions costs a single marker key.
func (s *Store) LoadUser(ctx context.Context, userID uint, stored map[uint]StoredReaction) error {
	pipe := s.client.TxPipeline()

	if len(stored) > 0 {
		values := make(map[string]any, len(stored))
		for postID, r := range stored {
			values[field(postID)] = encodeValue(r.Type, r.At.UnixMilli())
		}
		pipe.HSet(ctx, s.key(cachekeys.ReactionByUser(userID)), values)
	}
	pipe.Set(ctx, s.key(cachekeys.ReactionUserLoaded(userID)), "1", 0)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("reactions: load reactions of user %d: %w", userID, err)
	}
	return nil
}

// StoredReaction is a reaction as the database holds it.
type StoredReaction struct {
	Type string
	At   time.Time
}

// Forget clears a deleted post's tally. The per-user fields are left alone:
// enumerating them would mean touching every reactor of a viral post, and they
// cost nothing once nobody can read the post they point at.
func (s *Store) Forget(ctx context.Context, postID uint) error {
	err := s.client.Del(ctx,
		s.key(cachekeys.ReactionCounts(postID)),
		s.key(cachekeys.ReactionHydrated(postID)),
	).Err()
	if err != nil {
		return fmt.Errorf("reactions: forget post %d: %w", postID, err)
	}
	return nil
}

// pairToken is how a pending write is named in the dirty set.
func pairToken(postID, userID uint) string {
	return strconv.FormatUint(uint64(postID), 10) + ":" + strconv.FormatUint(uint64(userID), 10)
}

// tallyFromMap builds a Tally from HGETALL output, dropping zeroes and
// clamping negatives.
//
// A count that has drifted below zero should read as absent rather than as
// "-3 angry" on somebody's screen. The reconcile pass fixes the stored number;
// this only stops a wrong one reaching a viewer.
func tallyFromMap(raw map[string]string) Tally {
	tally := Tally{Counts: make(map[string]int64, len(raw))}

	for reaction, value := range raw {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		tally.Counts[reaction] = n
		tally.Total += n
	}

	return tally
}

// tallyFromFlat handles the alternating key/value list a Lua HGETALL returns.
func tallyFromFlat(flat []any) Tally {
	raw := make(map[string]string, len(flat)/2)

	for i := 0; i+1 < len(flat); i += 2 {
		field, ok1 := flat[i].(string)
		value, ok2 := flat[i+1].(string)
		if ok1 && ok2 {
			raw[field] = value
		}
	}

	return tallyFromMap(raw)
}
