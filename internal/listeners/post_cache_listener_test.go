package listeners_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/scriptertoufiq/go-mvc/internal/cachekeys"
	appevents "github.com/scriptertoufiq/go-mvc/internal/events"
	"github.com/scriptertoufiq/go-mvc/internal/listeners"
	"github.com/scriptertoufiq/go-mvc/internal/models"
	"github.com/scriptertoufiq/go-mvc/pkg/cache"
	"github.com/scriptertoufiq/go-mvc/pkg/events"
)

func newListener(t *testing.T) (*events.Dispatcher, cache.Cache, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	store, err := cache.NewRedis(cache.Options{
		Addr:        server.Addr(),
		DialTimeout: 2 * time.Second,
		Timeout:     2 * time.Second,
		PoolSize:    4,
	})
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	d := events.New()
	listeners.NewPostCacheListener(store, 5*time.Minute).Register(d)

	return d, store, server
}

func post(id uint, title string) *models.Post {
	p := &models.Post{UserID: 1, Title: title, Content: "body"}
	p.ID = id
	return p
}

func TestPostCreatedStoresThePost(t *testing.T) {
	d, store, _ := newListener(t)
	ctx := context.Background()

	d.Dispatch(ctx, appevents.PostCreated{Post: post(1, "Fresh")})

	var got models.Post
	found, err := store.Get(ctx, cachekeys.Post(1), &got)
	if err != nil || !found {
		t.Fatalf("expected the post to be cached, found=%v err=%v", found, err)
	}
	if got.Title != "Fresh" {
		t.Errorf("cached the wrong value: %+v", got)
	}
}

func TestPostUpdatedReplacesTheCachedCopy(t *testing.T) {
	d, store, _ := newListener(t)
	ctx := context.Background()

	d.Dispatch(ctx, appevents.PostCreated{Post: post(1, "Before")})
	d.Dispatch(ctx, appevents.PostUpdated{Post: post(1, "After")})

	var got models.Post
	if _, err := store.Get(ctx, cachekeys.Post(1), &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "After" {
		t.Errorf("cache still holds the old version: %q", got.Title)
	}
}

func TestPostDeletedRemovesTheCachedCopy(t *testing.T) {
	d, store, _ := newListener(t)
	ctx := context.Background()

	d.Dispatch(ctx, appevents.PostCreated{Post: post(7, "Doomed")})
	d.Dispatch(ctx, appevents.PostDeleted{PostID: 7, AuthorID: 1})

	var got models.Post
	found, err := store.Get(ctx, cachekeys.Post(7), &got)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found {
		t.Error("a deleted post is still cached")
	}
}

func TestEachPostIsCachedUnderItsOwnKey(t *testing.T) {
	d, store, _ := newListener(t)
	ctx := context.Background()

	d.Dispatch(ctx, appevents.PostCreated{Post: post(1, "One")})
	d.Dispatch(ctx, appevents.PostCreated{Post: post(2, "Two")})
	d.Dispatch(ctx, appevents.PostDeleted{PostID: 1, AuthorID: 1})

	var got models.Post
	if found, _ := store.Get(ctx, cachekeys.Post(1), &got); found {
		t.Error("post 1 should be gone")
	}
	if found, _ := store.Get(ctx, cachekeys.Post(2), &got); !found {
		t.Error("deleting post 1 must not disturb post 2")
	}
}

// Write-through must set an expiry. Without one a cache entry outlives every
// assumption made about it.
func TestCachedPostsCarryATTL(t *testing.T) {
	d, _, server := newListener(t)
	ctx := context.Background()

	d.Dispatch(ctx, appevents.PostCreated{Post: post(1, "Expiring")})

	if ttl := server.TTL(cachekeys.Post(1)); ttl != 5*time.Minute {
		t.Errorf("got ttl %v, want 5m", ttl)
	}
}

// A malformed event must be reported, not silently treated as a cache write.
func TestWrongEventTypeIsRejected(t *testing.T) {
	_, store, _ := newListener(t)
	ctx := context.Background()

	d := events.New()
	listeners.NewPostCacheListener(store, time.Minute).Register(d)

	// PostCreated with no post attached: the listener should error internally
	// and write nothing rather than panic.
	d.Dispatch(ctx, appevents.PostCreated{Post: nil})

	var got models.Post
	if found, _ := store.Get(ctx, cachekeys.Post(0), &got); found {
		t.Error("a malformed event should not have produced a cache entry")
	}
}
