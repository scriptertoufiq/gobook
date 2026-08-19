package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/scriptertoufiq/gobook/pkg/cache"
)

// A zero TTL must store the value with no expiry at all — not with an
// immediate one, which is what a careless translation to Redis would produce.
func TestZeroTTLStoresForever(t *testing.T) {
	server := miniredis.RunT(t)
	store, err := cache.NewRedis(cache.Options{
		Addr: server.Addr(), DialTimeout: 2 * time.Second, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	if err := store.Set(ctx, "permanent", map[string]int{"id": 1}, 0); err != nil {
		t.Fatalf("set: %v", err)
	}

	if ttl := server.TTL("permanent"); ttl != 0 {
		t.Errorf("expected no expiry, got ttl %v", ttl)
	}

	// Far beyond any plausible TTL — it must still be there.
	server.FastForward(48 * time.Hour)

	var got map[string]int
	found, err := store.Get(ctx, "permanent", &got)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !found {
		t.Fatal("a zero-TTL entry expired, but it should never expire")
	}
	if got["id"] != 1 {
		t.Errorf("value came back wrong: %v", got)
	}
}
