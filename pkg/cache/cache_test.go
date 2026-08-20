package cache_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/scriptertoufiq/gobook/pkg/cache"
)

type post struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
}

// newRedis starts an in-process Redis and returns a cache pointed at it, so
// these tests need no server and no network.
func newRedis(t *testing.T, prefix string) (*cache.Redis, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)

	store, err := cache.NewRedis(cache.Options{
		Addr:        server.Addr(),
		Prefix:      prefix,
		DialTimeout: 2 * time.Second,
		Timeout:     2 * time.Second,
		PoolSize:    4,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	return store, server
}

func TestNewRedisFailsWhenServerIsUnreachable(t *testing.T) {
	// Closed port: the PING in NewRedis is what turns a bad address into a boot
	// failure rather than a surprise on the first cached read.
	_, err := cache.NewRedis(cache.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: time.Second,
		Timeout:     time.Second,
	})
	if err == nil {
		t.Fatal("expected an error for an unreachable server")
	}
}

func TestMissIsNotAnError(t *testing.T) {
	store, _ := newRedis(t, "")
	ctx := context.Background()

	var got post
	found, err := store.Get(ctx, "posts:1", &got)
	if err != nil {
		t.Fatalf("a miss must not be an error, got %v", err)
	}
	if found {
		t.Fatal("nothing was stored, so this must report a miss")
	}
}

func TestSetThenGetRoundTripsTheValue(t *testing.T) {
	store, _ := newRedis(t, "")
	ctx := context.Background()

	want := post{ID: 7, Title: "Rotation is a critical section"}
	if err := store.Set(ctx, "posts:7", want, time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}

	var got post
	found, err := store.Get(ctx, "posts:7", &got)
	if err != nil || !found {
		t.Fatalf("expected a hit, got found=%v err=%v", found, err)
	}
	if got != want {
		t.Errorf("round trip changed the value: got %+v, want %+v", got, want)
	}
}

func TestEntryExpiresAfterTTL(t *testing.T) {
	store, server := newRedis(t, "")
	ctx := context.Background()

	if err := store.Set(ctx, "short", post{ID: 1}, 30*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}

	// miniredis has a controllable clock, so expiry needs no sleep.
	server.FastForward(31 * time.Second)

	var got post
	found, _ := store.Get(ctx, "short", &got)
	if found {
		t.Error("entry should have expired")
	}
}

func TestPrefixNamespacesKeys(t *testing.T) {
	store, server := newRedis(t, "gobook")
	ctx := context.Background()

	if err := store.Set(ctx, "posts:1", post{ID: 1}, time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}

	// The prefix must be applied in Redis itself — that is what lets two apps
	// share one instance.
	if !server.Exists("gobook:posts:1") {
		t.Errorf("expected key gobook:posts:1, keys are %v", server.Keys())
	}
}

func TestDeleteRemovesKeysAndIgnoresAbsentOnes(t *testing.T) {
	store, _ := newRedis(t, "")
	ctx := context.Background()

	_ = store.Set(ctx, "a", post{ID: 1}, time.Minute)

	if err := store.Delete(ctx, "a", "never-existed"); err != nil {
		t.Fatalf("deleting an absent key must not error: %v", err)
	}

	var got post
	if found, _ := store.Get(ctx, "a", &got); found {
		t.Error("key should be gone")
	}
}

func TestDeleteByPrefixClearsOnlyTheMatchingFamily(t *testing.T) {
	store, _ := newRedis(t, "gobook")
	ctx := context.Background()

	for _, k := range []string{"posts:list:1", "posts:list:2", "posts:show:9"} {
		_ = store.Set(ctx, k, post{ID: 1}, time.Minute)
	}
	_ = store.Set(ctx, "users:list:1", post{ID: 2}, time.Minute)

	if err := store.DeleteByPrefix(ctx, "posts:list"); err != nil {
		t.Fatalf("delete by prefix: %v", err)
	}

	var got post
	for _, gone := range []string{"posts:list:1", "posts:list:2"} {
		if found, _ := store.Get(ctx, gone, &got); found {
			t.Errorf("%s should have been cleared", gone)
		}
	}
	for _, kept := range []string{"posts:show:9", "users:list:1"} {
		if found, _ := store.Get(ctx, kept, &got); !found {
			t.Errorf("%s must survive — it is outside the prefix", kept)
		}
	}
}

func TestUndecodableEntryIsDroppedRatherThanReturned(t *testing.T) {
	store, server := newRedis(t, "")
	ctx := context.Background()

	// Simulates a cached struct whose shape changed under a live key.
	if err := server.Set("broken", "this is not json"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var got post
	found, err := store.Get(ctx, "broken", &got)
	if err != nil || found {
		t.Fatalf("a corrupt entry must read as a miss, got found=%v err=%v", found, err)
	}
	if server.Exists("broken") {
		t.Error("the corrupt entry should have been deleted, not left to fail forever")
	}
}

func TestRememberComputesOnMissAndCachesTheResult(t *testing.T) {
	store, _ := newRedis(t, "")
	ctx := context.Background()

	calls := 0
	compute := func() (post, error) {
		calls++
		return post{ID: 3, Title: "computed"}, nil
	}

	first, err := cache.Remember(ctx, store, "posts:3", time.Minute, compute)
	if err != nil {
		t.Fatalf("remember: %v", err)
	}

	second, err := cache.Remember(ctx, store, "posts:3", time.Minute, compute)
	if err != nil {
		t.Fatalf("remember: %v", err)
	}

	if calls != 1 {
		t.Errorf("the second call should have been served from cache, compute ran %d times", calls)
	}
	if first != second {
		t.Errorf("cached value differs from the computed one: %+v vs %+v", first, second)
	}
}

func TestRememberPropagatesComputeErrors(t *testing.T) {
	store, _ := newRedis(t, "")
	ctx := context.Background()

	sentinel := errors.New("database exploded")
	_, err := cache.Remember(ctx, store, "posts:4", time.Minute, func() (post, error) {
		return post{}, sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the compute error to surface, got %v", err)
	}
}

func TestRememberStillAnswersWhenTheCacheIsDown(t *testing.T) {
	store, server := newRedis(t, "")
	ctx := context.Background()

	server.Close() // Redis disappears mid-flight

	got, err := cache.Remember(ctx, store, "posts:5", time.Minute, func() (post, error) {
		return post{ID: 5, Title: "from the database"}, nil
	})
	if err != nil {
		t.Fatalf("a dead cache must not fail the request: %v", err)
	}
	if got.ID != 5 {
		t.Errorf("expected the computed value, got %+v", got)
	}
}

func TestNullCacheAlwaysMissesAndNeverErrors(t *testing.T) {
	store := cache.NewNull()
	ctx := context.Background()

	if err := store.Set(ctx, "k", post{ID: 1}, time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}

	var got post
	found, err := store.Get(ctx, "k", &got)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found {
		t.Error("the null cache must never report a hit")
	}

	// Remember must still work against it — that is what lets callers stay
	// branch-free when caching is switched off.
	value, err := cache.Remember(ctx, store, "k", time.Minute, func() (post, error) {
		return post{ID: 9}, nil
	})
	if err != nil || value.ID != 9 {
		t.Errorf("expected the computed value, got %+v err=%v", value, err)
	}
}

func TestKeyJoinsPartsWithColons(t *testing.T) {
	if got := cache.Key("posts", "list", 2); got != "posts:list:2" {
		t.Errorf("got %q, want posts:list:2", got)
	}
}

func TestBumpStartsAtOneAndClimbs(t *testing.T) {
	store, _ := newRedis(t, "gen")
	ctx := context.Background()

	first, err := store.Bump(ctx, "posts:list:gen")
	if err != nil {
		t.Fatalf("bump: %v", err)
	}
	if first != 1 {
		t.Errorf("a counter that did not exist should start at 1, got %d", first)
	}

	second, _ := store.Bump(ctx, "posts:list:gen")
	if second != 2 {
		t.Errorf("got %d, want 2", second)
	}
}

func TestGenerationOfAnUntouchedCounterIsZero(t *testing.T) {
	store, _ := newRedis(t, "gen")
	ctx := context.Background()

	// Nothing has invalidated anything yet — that is an answer, not a fault.
	n, err := store.Generation(ctx, "posts:list:gen")
	if err != nil {
		t.Fatalf("generation: %v", err)
	}
	if n != 0 {
		t.Errorf("got %d, want 0", n)
	}
}

func TestGenerationReadsWhatBumpWrote(t *testing.T) {
	store, _ := newRedis(t, "gen")
	ctx := context.Background()

	for range 3 {
		if _, err := store.Bump(ctx, "posts:list:gen"); err != nil {
			t.Fatalf("bump: %v", err)
		}
	}

	n, err := store.Generation(ctx, "posts:list:gen")
	if err != nil {
		t.Fatalf("generation: %v", err)
	}
	if n != 3 {
		t.Errorf("got %d, want 3", n)
	}
}

// Invalidation must not get slower as the cache fills — the whole reason this
// replaced a prefix scan.
func TestBumpCostDoesNotGrowWithTheKeyspace(t *testing.T) {
	store, server := newRedis(t, "gen")
	ctx := context.Background()

	for i := range 5000 {
		if err := server.Set(fmt.Sprintf("unrelated:%d", i), "x"); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// A scan would visit all 5000; this touches exactly one key.
	if _, err := store.Bump(ctx, "posts:list:gen"); err != nil {
		t.Fatalf("bump: %v", err)
	}
	if n, _ := store.Generation(ctx, "posts:list:gen"); n != 1 {
		t.Errorf("got %d, want 1", n)
	}
}

// The reason single-flight exists: a hot key that has just been invalidated is
// missed by every concurrent reader at once, and without coalescing every one
// of them runs the same query.
func TestConcurrentMissesRunTheQueryOnce(t *testing.T) {
	store, _ := newRedis(t, "flight")
	ctx := context.Background()

	var computed atomic.Int64
	var wg sync.WaitGroup

	const readers = 50
	release := make(chan struct{})

	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release // all miss at the same instant

			_, err := cache.Remember(ctx, store, "posts:hot", time.Minute,
				func() (post, error) {
					computed.Add(1)
					// Long enough that the others are certainly waiting.
					time.Sleep(40 * time.Millisecond)
					return post{ID: 1, Title: "expensive"}, nil
				})
			if err != nil {
				t.Errorf("remember: %v", err)
			}
		}()
	}

	close(release)
	wg.Wait()

	if n := computed.Load(); n != 1 {
		t.Errorf("%d readers caused %d queries, want 1", readers, n)
	}
}

// Every waiter must receive the value, not just the one that computed it.
func TestEveryWaiterGetsTheComputedValue(t *testing.T) {
	store, _ := newRedis(t, "flight")
	ctx := context.Background()

	var wg sync.WaitGroup
	results := make([]post, 20)
	release := make(chan struct{})

	for i := range results {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			<-release
			got, err := cache.Remember(ctx, store, "posts:shared", time.Minute,
				func() (post, error) {
					time.Sleep(30 * time.Millisecond)
					return post{ID: 99, Title: "one value for everybody"}, nil
				})
			if err != nil {
				t.Errorf("remember: %v", err)
				return
			}
			results[slot] = got
		}(i)
	}

	close(release)
	wg.Wait()

	for i, got := range results {
		if got.ID != 99 || got.Title != "one value for everybody" {
			t.Errorf("waiter %d got %+v", i, got)
		}
	}
}

// A failing computation must reach every waiter rather than leaving some of
// them hanging on a value that was never produced.
func TestComputeErrorReachesEveryWaiter(t *testing.T) {
	store, _ := newRedis(t, "flight")
	ctx := context.Background()

	sentinel := errors.New("database is down")
	var wg sync.WaitGroup
	failures := atomic.Int64{}
	release := make(chan struct{})

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			_, err := cache.Remember(ctx, store, "posts:failing", time.Minute,
				func() (post, error) {
					time.Sleep(20 * time.Millisecond)
					return post{}, sentinel
				})
			if errors.Is(err, sentinel) {
				failures.Add(1)
			}
		}()
	}

	close(release)
	wg.Wait()

	if n := failures.Load(); n != 10 {
		t.Errorf("%d of 10 waiters saw the error, want all of them", n)
	}
}
