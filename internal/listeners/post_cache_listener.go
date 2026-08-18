// Package listeners wires reactions onto application events. Each listener is
// one side effect, registered at boot, so the code that causes an event never
// has to know who is watching.
package listeners

import (
	"context"
	"fmt"
	"time"

	"github.com/scriptertoufiq/go-mvc/internal/cachekeys"
	appevents "github.com/scriptertoufiq/go-mvc/internal/events"
	"github.com/scriptertoufiq/go-mvc/pkg/cache"
	"github.com/scriptertoufiq/go-mvc/pkg/events"
)

// PostCacheListener keeps the post cache in step with the database.
//
// Creates and updates write through — the value that was just persisted is
// stored, so the next reader is served without a query. Deletes remove the key.
//
// The trade-off in writing through rather than simply invalidating: two
// updates racing can, in principle, leave the cache holding whichever write
// landed second in Redis even though the database kept the other. The window
// is the gap between the two operations, and it self-heals at the TTL. What it
// buys is that an edited post is immediately readable from cache instead of
// every reader after an edit paying for a database round trip.
type PostCacheListener struct {
	cache cache.Cache
	ttl   time.Duration
}

func NewPostCacheListener(store cache.Cache, ttl time.Duration) *PostCacheListener {
	return &PostCacheListener{cache: store, ttl: ttl}
}

// Register subscribes this listener to the post lifecycle events.
//
// One place that says what reacts to what — the thing you read to answer "why
// did the cache change?".
func (l *PostCacheListener) Register(d *events.Dispatcher) {
	d.Listen(appevents.PostCreatedName, "post-cache", l.onCreated)
	d.Listen(appevents.PostUpdatedName, "post-cache", l.onUpdated)
	d.Listen(appevents.PostDeletedName, "post-cache", l.onDeleted)
}

func (l *PostCacheListener) onCreated(ctx context.Context, event events.Event) error {
	created, ok := event.(appevents.PostCreated)
	if !ok {
		return fmt.Errorf("post-cache: expected PostCreated, got %T", event)
	}
	if created.Post == nil {
		return fmt.Errorf("post-cache: PostCreated carried no post")
	}

	return l.store(ctx, created.Post.ID, *created.Post)
}

func (l *PostCacheListener) onUpdated(ctx context.Context, event events.Event) error {
	updated, ok := event.(appevents.PostUpdated)
	if !ok {
		return fmt.Errorf("post-cache: expected PostUpdated, got %T", event)
	}
	if updated.Post == nil {
		return fmt.Errorf("post-cache: PostUpdated carried no post")
	}

	return l.store(ctx, updated.Post.ID, *updated.Post)
}

func (l *PostCacheListener) onDeleted(ctx context.Context, event events.Event) error {
	deleted, ok := event.(appevents.PostDeleted)
	if !ok {
		return fmt.Errorf("post-cache: expected PostDeleted, got %T", event)
	}

	if err := l.cache.Delete(ctx, cachekeys.Post(deleted.PostID)); err != nil {
		return fmt.Errorf("removing post %d from cache: %w", deleted.PostID, err)
	}

	return l.clearListings(ctx)
}

// store writes the post through to the cache, then clears the listings — a new
// or edited post changes what a page of results contains, and those pages are
// keyed by page number, search term and sort, so they cannot be updated
// individually.
func (l *PostCacheListener) store(ctx context.Context, id uint, value any) error {
	if err := l.cache.Set(ctx, cachekeys.Post(id), value, l.ttl); err != nil {
		return fmt.Errorf("caching post %d: %w", id, err)
	}

	return l.clearListings(ctx)
}

// clearListings drops every cached page of the post listing.
//
// Nothing caches listings yet, so today this is a no-op against an empty
// prefix. It is here because the moment listing caching is added, forgetting
// this is exactly the bug that ships: a new post that never appears.
func (l *PostCacheListener) clearListings(ctx context.Context) error {
	if err := l.cache.DeleteByPrefix(ctx, cachekeys.PostListPrefix()); err != nil {
		return fmt.Errorf("clearing cached post listings: %w", err)
	}
	return nil
}
