package ratelimit_test

import (
	"sync"
	"testing"
	"time"

	"github.com/scriptertoufiq/go-mvc/pkg/ratelimit"
)

// clock is a manually advanced time source, so these tests are deterministic
// and take no wall-clock time.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock {
	return &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newLimiter(limit int, window time.Duration) (*ratelimit.Limiter, *clock) {
	c := newClock()
	l := ratelimit.New(limit, window)
	l.SetClock(c.Now)
	return l, c
}

func TestAllowsUpToTheLimitThenBlocks(t *testing.T) {
	l, _ := newLimiter(3, time.Minute)

	for i := 1; i <= 3; i++ {
		if res := l.Allow("ip:1.2.3.4"); !res.Allowed {
			t.Fatalf("request %d should have been allowed", i)
		}
	}

	res := l.Allow("ip:1.2.3.4")
	if res.Allowed {
		t.Fatal("the 4th request should have been blocked")
	}
	if res.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", res.Remaining)
	}
	if res.RetryAfter <= 0 {
		t.Error("a blocked result must report a positive RetryAfter")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l, _ := newLimiter(1, time.Minute)

	if res := l.Allow("ip:1.1.1.1"); !res.Allowed {
		t.Fatal("first key should be allowed")
	}
	if res := l.Allow("ip:1.1.1.1"); res.Allowed {
		t.Fatal("first key should now be exhausted")
	}
	if res := l.Allow("ip:2.2.2.2"); !res.Allowed {
		t.Fatal("a different key must have its own budget")
	}
}

func TestTokensRefillOverTime(t *testing.T) {
	l, c := newLimiter(60, time.Minute) // one token per second

	for range 60 {
		l.Allow("k")
	}
	if res := l.Allow("k"); res.Allowed {
		t.Fatal("bucket should be empty")
	}

	c.Advance(time.Second) // exactly one token back
	if res := l.Allow("k"); !res.Allowed {
		t.Fatal("one token should have refilled after a second")
	}
	if res := l.Allow("k"); res.Allowed {
		t.Fatal("only one token should have refilled")
	}

	c.Advance(10 * time.Second)
	for i := range 10 {
		if res := l.Allow("k"); !res.Allowed {
			t.Fatalf("token %d of 10 should have refilled", i+1)
		}
	}
}

// The bucket must never exceed its capacity, or a caller who idles for an hour
// could burst an hour's worth of requests at once.
func TestRefillIsCappedAtTheLimit(t *testing.T) {
	l, c := newLimiter(5, time.Minute)

	l.Allow("k")
	c.Advance(24 * time.Hour)

	for i := 1; i <= 5; i++ {
		if res := l.Allow("k"); !res.Allowed {
			t.Fatalf("request %d should be allowed after a long idle", i)
		}
	}
	if res := l.Allow("k"); res.Allowed {
		t.Fatal("bucket refilled beyond its capacity")
	}
}

// A fixed-window limiter allows 2x the limit across a boundary. A bucket must
// not: 10/minute means at most ~10 in any 60-second stretch.
func TestNoDoubleBurstAcrossAWindowBoundary(t *testing.T) {
	l, c := newLimiter(10, time.Minute)

	allowed := 0
	for range 10 {
		if l.Allow("k").Allowed {
			allowed++
		}
	}

	// Jump to the far edge of the window and try to spend a second full budget.
	c.Advance(59 * time.Second)
	for range 10 {
		if l.Allow("k").Allowed {
			allowed++
		}
	}

	// 10 initial + ~9 refilled over 59s. A fixed window would give 20.
	if allowed > 19 {
		t.Errorf("allowed %d requests inside ~60s for a limit of 10", allowed)
	}
}

func TestEvictionDropsIdleKeysOnly(t *testing.T) {
	l, c := newLimiter(5, time.Minute) // idleTTL = 10 windows = 10 minutes

	l.Allow("stale")
	c.Advance(11 * time.Minute)
	l.Allow("fresh")

	if got := l.Len(); got != 2 {
		t.Fatalf("expected 2 tracked keys before eviction, got %d", got)
	}

	l.StartJanitor(time.Millisecond)
	defer l.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if l.Len() == 1 {
			return // stale evicted, fresh kept
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("janitor did not evict the idle key; %d keys remain", l.Len())
}

func TestStopIsIdempotent(t *testing.T) {
	l, _ := newLimiter(1, time.Minute)
	l.StartJanitor(time.Minute)

	l.Stop()
	l.Stop() // must not panic on a closed channel
}

func TestConcurrentAllowIsRaceFree(t *testing.T) {
	l, _ := newLimiter(1000, time.Minute)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 20 {
				l.Allow("shared")
				l.Allow(string(rune('a' + n%26)))
			}
		}(i)
	}
	wg.Wait()

	// 50 goroutines * 20 = 1000 on the shared key, exactly the limit.
	if res := l.Allow("shared"); res.Allowed {
		t.Error("the shared key should be exhausted after 1000 requests")
	}
}
