// Package cache is the application's key/value cache.
//
// It is an interface for the same reason mailer.Mailer is: services should be
// testable without a live server, and the one place that knows about Redis
// should be redis.go. A deployment with caching switched off gets Null, which
// satisfies the same contract by always missing — so calling code needs no
// `if cache != nil` branch anywhere.
package cache

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
)

// flights coalesces concurrent misses on the same key.
//
// Without it, invalidating a hot key is actively harmful: every reader misses
// at the same instant and every one of them runs the same query. Measured on a
// comment thread taking twenty writes a second, that turned 77 queries into
// 1,546 and dropped reads from 4,743/s to 150/s — the busier the thread, the
// worse it behaved. With coalescing, N concurrent misses become one query and
// N-1 waiters.
//
// Package-level, which assumes one cache instance per key namespace. That holds
// here; two Cache values with different prefixes sharing a logical key would
// share a computed value, which would be wrong.
var flights singleflight.Group

// Cache stores JSON-encoded values under string keys.
//
// Every method takes a context so a slow cache can be abandoned rather than
// becoming slower than the database it is meant to spare.
type Cache interface {
	// Get decodes the value at key into dest. The bool reports a hit; a miss is
	// (false, nil), not an error — missing is the normal case, not a fault.
	Get(ctx context.Context, key string, dest any) (bool, error)

	// Set stores value under key. A ttl of 0 means no expiry.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes keys. Absent keys are not an error.
	Delete(ctx context.Context, keys ...string) error

	// DeleteByPrefix removes every key beginning with prefix.
	//
	// NEVER call this on a request path. It scans the whole keyspace — the
	// server filters, so the cost is the total number of keys whether or not
	// any of them match. Measured here, running it per post creation took
	// throughput from 828 to 15 requests a second as unrelated keys
	// accumulated. To invalidate a family of cached reads, bump a generation
	// instead. This stays for administrative one-offs.
	DeleteByPrefix(ctx context.Context, prefix string) error

	// Bump increments a counter and returns its new value, creating it at 1.
	//
	// The cheap way to invalidate a whole family of entries: cached keys embed
	// the current generation, so raising it orphans every one of them at once
	// and they expire on their own. One command, no scan, cost independent of
	// how large the cache has grown.
	Bump(ctx context.Context, key string) (int64, error)

	// Generation reads a counter without changing it. A missing counter is 0,
	// not an error — nothing has invalidated anything yet.
	Generation(ctx context.Context, key string) (int64, error)

	// Ping verifies the backend is reachable.
	Ping(ctx context.Context) error

	// Close releases the connection pool.
	Close() error
}

// Source says where a value came from. Handlers surface it so a caller can
// tell a cached answer from a freshly computed one.
type Source string

const (
	// FromCache — the value was already stored.
	FromCache Source = "cache"
	// FromOrigin — the cache missed and the value was computed, then stored.
	FromOrigin Source = "database"
)

// IsHit reports whether the value was served from the cache.
func (s Source) IsHit() bool { return s == FromCache }

// Remember returns the cached value at key, computing and storing it on a miss.
//
// A free function rather than a method so it can be generic: the caller gets a
// typed value back instead of passing a pointer and hoping the decode matched.
func Remember[T any](
	ctx context.Context,
	c Cache,
	key string,
	ttl time.Duration,
	compute func() (T, error),
) (T, error) {
	value, _, err := RememberFrom(ctx, c, key, ttl, compute)
	return value, err
}

// RememberFrom is Remember, and additionally reports which path answered.
//
// A cache failure is never fatal here. If Redis is unreachable the value is
// still computed and returned — a degraded cache must not degrade the API —
// and the reported Source is FromOrigin, which is the truth: that answer did
// come from the database.
func RememberFrom[T any](
	ctx context.Context,
	c Cache,
	key string,
	ttl time.Duration,
	compute func() (T, error),
) (T, Source, error) {
	var cached T

	found, err := c.Get(ctx, key, &cached)
	if err == nil && found {
		return cached, FromCache, nil
	}

	// From here the value has to be computed — and everybody who missed at the
	// same moment would otherwise compute it independently. Only one does.
	result, err, _ := flights.Do(key, func() (any, error) {
		// A flight that just finished may already have stored it, so look once
		// more before paying for the query. One extra read per flight, not per
		// waiter.
		var fresh T
		if found, err := c.Get(ctx, key, &fresh); err == nil && found {
			return fresh, nil
		}

		value, err := compute()
		if err != nil {
			return value, err
		}

		// Deliberately ignored: failing to cache is not failing to answer.
		_ = c.Set(ctx, key, value, ttl)

		return value, nil
	})
	if err != nil {
		var zero T
		if typed, ok := result.(T); ok {
			zero = typed
		}
		return zero, FromOrigin, err
	}

	value, ok := result.(T)
	if !ok {
		// Two callers asked for the same key expecting different types, which
		// is a key-naming bug rather than a cache failure. Compute directly so
		// the request still succeeds, and leave the shared entry alone.
		return compute2(compute)
	}

	return value, FromOrigin, nil
}

// compute2 is the escape hatch for a key collision between two value types.
func compute2[T any](compute func() (T, error)) (T, Source, error) {
	value, err := compute()
	return value, FromOrigin, err
}

// Key joins parts into a namespaced cache key: Key("posts", "list", 2) is
// "posts:list:2". Centralised so key formats stay consistent and greppable.
func Key(parts ...any) string {
	return join(parts)
}
