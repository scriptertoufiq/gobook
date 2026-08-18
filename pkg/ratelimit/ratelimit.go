// Package ratelimit provides an in-process, per-key token bucket.
//
// Token bucket rather than a fixed window because a fixed window lets a caller
// fire 2x the limit across a boundary — all of window N's budget at the last
// instant, then all of window N+1's at the first. A bucket refills smoothly, so
// the rate holds across any interval.
//
// State lives in this process. That is the right trade for one instance; behind
// a load balancer each replica enforces its own share, so N replicas allow
// roughly N times the configured rate. Move the buckets to Redis when that
// stops being acceptable — the Limiter interface is the seam.
package ratelimit

import (
	"math"
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	limit   int           // tokens per window, and the burst capacity
	window  time.Duration //
	refill  float64       // tokens per second
	idleTTL time.Duration // how long an untouched bucket survives eviction

	now      func() time.Time // injectable so tests need no sleeps
	stop     chan struct{}
	stopOnce sync.Once
}

type bucket struct {
	tokens float64
	last   time.Time
}

// Result describes one decision, shaped for the response headers.
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration // how long until one token is available
	Reset      time.Time     // when the bucket is back to full
}

func New(limit int, window time.Duration) *Limiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}

	return &Limiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
		window:  window,
		refill:  float64(limit) / window.Seconds(),
		// Ten idle windows means a bucket is long since full before it is
		// dropped, so eviction can never hand anyone a fresh allowance.
		idleTTL: window * 10,
		now:     time.Now,
		stop:    make(chan struct{}),
	}
}

// Allow consumes one token for key and reports the outcome.
func (l *Limiter) Allow(key string) Result {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.limit), last: now}
		l.buckets[key] = b
	} else if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = math.Min(float64(l.limit), b.tokens+elapsed*l.refill)
		b.last = now
	}

	result := Result{Limit: l.limit}

	if b.tokens >= 1 {
		b.tokens--
		result.Allowed = true
	} else {
		result.RetryAfter = l.secondsFor(1 - b.tokens)
	}

	result.Remaining = int(b.tokens)
	result.Reset = now.Add(l.secondsFor(float64(l.limit) - b.tokens))

	return result
}

// secondsFor converts a token deficit into the time needed to refill it.
func (l *Limiter) secondsFor(tokens float64) time.Duration {
	if tokens <= 0 {
		return 0
	}
	return time.Duration(tokens / l.refill * float64(time.Second))
}

// StartJanitor evicts idle buckets on an interval.
//
// This is not housekeeping — it is the memory bound. Without it, one key per
// spoofed source address grows the map forever, so the limiter itself becomes
// the denial-of-service vector it exists to prevent.
func (l *Limiter) StartJanitor(interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				l.evictIdle()
			case <-l.stop:
				return
			}
		}
	}()
}

// Stop halts the janitor. Safe to call more than once.
func (l *Limiter) Stop() {
	l.stopOnce.Do(func() { close(l.stop) })
}

func (l *Limiter) evictIdle() {
	cutoff := l.now().Add(-l.idleTTL)

	l.mu.Lock()
	defer l.mu.Unlock()

	for key, b := range l.buckets {
		if b.last.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}

// Len reports how many keys are being tracked. Exposed for tests and metrics.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// SetClock replaces the time source. Tests only.
func (l *Limiter) SetClock(now func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.now = now
}
