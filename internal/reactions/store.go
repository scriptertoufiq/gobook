// Package reactions is the hot tier: Redis holds the authoritative state of
// every reaction and answers every read, while a background flusher folds
// changes into MySQL in batches.
//
// It keeps its own Redis client rather than going through pkg/cache. That
// package is a JSON get/set/delete cache by design; reactions need hashes,
// sets and a Lua script, and widening the general cache to fit one feature
// would make it worse at the job it already does.
package reactions

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/scriptertoufiq/go-mvc/internal/cachekeys"
	"github.com/scriptertoufiq/go-mvc/internal/repositories"
)

// None is written when somebody takes their reaction back.
//
// Removing writes this sentinel instead of deleting the key, so an absent key
// means one thing only: nobody has looked this person up yet. Without it, "has
// no reaction" and "not loaded" are indistinguishable, and every page view by
// a non-reactor — most of them — would re-query MySQL forever.
const None = "-"

// A stored reaction is "type|unixMillis" — the choice, and when the person
// made it.
//
// The timestamp is what lets a reaction queued on a phone with no signal be
// replayed later without stamping on a newer one made since. Keeping it in the
// same string rather than a second key is what keeps the whole update inside
// one script.
const valueSeparator = "|"

func encodeValue(reaction string, atMillis int64) string {
	return reaction + valueSeparator + strconv.FormatInt(atMillis, 10)
}

// decodeValue splits a stored value. A value with no separator is read as a
// bare reaction from before timestamps existed, at time zero, so anything
// newer replaces it.
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
// permanently, because nothing on the read path ever recomputes them. Redis
// runs a script to completion with nothing in between.
var setScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
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

redis.call('SET', KEYS[1], new .. '|' .. ARGV[3])
redis.call('SADD', KEYS[3], ARGV[2])

return {'applied', new, redis.call('HGETALL', KEYS[2])}
`)

// hydrateScript writes a post's baseline from the database and marks it
// loaded, but only if nobody has done so already.
//
// Atomic for the same reason the set script is: between checking the marker and
// writing the baseline, a live reaction could increment a field, and a plain
// HSET would then overwrite it back down. Doing both inside one script means an
// increment can only land before or after, never in the middle.
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

// Set applies a reaction made at actedAt, and reports the person's resulting
// choice along with the fresh tally.
//
// applied is false when the action was older than what is already stored — a
// queued offline reaction arriving after a newer one. Nothing changes in that
// case; the caller is told so it can stop retrying.
func (s *Store) Set(
	ctx context.Context,
	postID, userID uint,
	reaction string,
	actedAt time.Time,
) (mine string, tally Tally, applied bool, err error) {
	keys := []string{
		s.key(cachekeys.ReactionUser(postID, userID)),
		s.key(cachekeys.ReactionCounts(postID)),
		s.key(cachekeys.ReactionDirty()),
	}

	raw, err := s.set.Run(ctx, s.client, keys,
		reaction, pairToken(postID, userID), actedAt.UnixMilli()).Slice()
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

// Mine reads one person's choice. The bool reports whether Redis knew — false
// means nobody has looked this person up yet and MySQL still has the answer.
func (s *Store) Mine(ctx context.Context, postID, userID uint) (string, bool, error) {
	value, err := s.client.Get(ctx, s.key(cachekeys.ReactionUser(postID, userID))).Result()
	switch {
	case errors.Is(err, redis.Nil):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("reactions: read reaction of user %d on post %d: %w", userID, postID, err)
	}

	reaction, _ := decodeValue(value)
	if reaction == None {
		return "", true, nil
	}
	return reaction, true, nil
}

// Hydrate seeds a post's tally from the database and marks it loaded, unless
// that has already happened.
//
// It deliberately does not touch the dirty set: nothing changed, it was only
// read.
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

// RememberMine caches one person's stored choice, including the fact that they
// have none. Pass "" for no reaction and the sentinel is written for you.
//
// storedAt is when the database row was last written, so a queued offline
// action older than it is still correctly rejected.
func (s *Store) RememberMine(
	ctx context.Context,
	postID, userID uint,
	reaction string,
	storedAt time.Time,
) error {
	if reaction == "" {
		reaction = None
	}

	key := s.key(cachekeys.ReactionUser(postID, userID))
	if err := s.client.Set(ctx, key, encodeValue(reaction, storedAt.UnixMilli()), 0).Err(); err != nil {
		return fmt.Errorf("reactions: cache reaction of user %d on post %d: %w", userID, postID, err)
	}
	return nil
}

// Forget clears everything cached for a post. Called when a post is deleted —
// its rows go with it via the foreign key, so the tally must go too.
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

// parsePair reverses pairToken.
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

// tallyFromMap builds a Tally from HGETALL output, dropping zeroes and
// clamping negatives.
//
// A count that has drifted below zero should read as absent rather than as
// "-3 angry" on somebody's screen. The nightly reconcile fixes the stored
// number; this only stops a wrong one reaching a viewer.
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
