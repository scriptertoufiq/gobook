package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Options is everything the Redis cache needs, kept free of the config package
// so pkg stays importable from anywhere.
type Options struct {
	Addr     string
	Username string
	Password string
	DB       int

	// Prefix namespaces every key, so one Redis instance can serve several
	// applications — or several environments — without them colliding.
	Prefix string

	// DialTimeout bounds the connection attempt at boot.
	DialTimeout time.Duration
	// Timeout bounds individual commands. A cache that answers slower than the
	// database has stopped being a cache.
	Timeout time.Duration

	PoolSize int
}

// Redis is a Cache backed by a Redis server.
type Redis struct {
	client *redis.Client
	prefix string
}

var _ Cache = (*Redis)(nil)

// NewRedis opens the pool and verifies it with a PING, so a bad address or
// password fails at boot rather than on the first cached read — the same
// contract database.Connect follows.
func NewRedis(opts Options) (*Redis, error) {
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
		return nil, fmt.Errorf("cache: ping redis at %s: %w", opts.Addr, err)
	}

	return &Redis{client: client, prefix: opts.Prefix}, nil
}

// key applies the configured namespace.
func (r *Redis) key(k string) string {
	if r.prefix == "" {
		return k
	}
	return r.prefix + ":" + k
}

// Get decodes the stored JSON into dest. redis.Nil — the miss — is translated
// to (false, nil), because a miss is an ordinary outcome, not a failure.
func (r *Redis) Get(ctx context.Context, key string, dest any) (bool, error) {
	raw, err := r.client.Get(ctx, r.key(key)).Bytes()
	switch {
	case errors.Is(err, redis.Nil):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("cache: get %s: %w", key, err)
	}

	if err := json.Unmarshal(raw, dest); err != nil {
		// A value we cannot decode is worse than no value: drop it so the next
		// caller recomputes instead of hitting the same broken entry forever.
		// This happens when a cached struct's shape changes under a live key.
		_ = r.client.Del(ctx, r.key(key)).Err()
		log.Printf("cache: discarding undecodable entry %s: %v", key, err)
		return false, nil
	}

	return true, nil
}

func (r *Redis) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache: encode %s: %w", key, err)
	}

	if err := r.client.Set(ctx, r.key(key), encoded, ttl).Err(); err != nil {
		return fmt.Errorf("cache: set %s: %w", key, err)
	}
	return nil
}

func (r *Redis) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	prefixed := make([]string, len(keys))
	for i, k := range keys {
		prefixed[i] = r.key(k)
	}

	if err := r.client.Del(ctx, prefixed...).Err(); err != nil {
		return fmt.Errorf("cache: delete: %w", err)
	}
	return nil
}

// DeleteByPrefix removes every key under a prefix.
//
// SCAN rather than KEYS, deliberately: KEYS walks the entire keyspace in one
// blocking call, which on a busy server stalls every other client. SCAN is
// cursor-based and yields between batches, so this stays safe to call from a
// request path.
func (r *Redis) DeleteByPrefix(ctx context.Context, prefix string) error {
	pattern := r.key(prefix) + "*"

	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 256).Result()
		if err != nil {
			return fmt.Errorf("cache: scan %s: %w", pattern, err)
		}

		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache: delete scanned keys: %w", err)
			}
		}

		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	return r.client.Close()
}
