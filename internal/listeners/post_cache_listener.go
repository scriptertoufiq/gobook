// Package listeners wires reactions onto application events. Each listener is
// one side effect, registered at boot, so the code that causes an event never
// has to know who is watching.
package listeners

import (
	"context"
	"fmt"
	"time"

	"github.com/scriptertoufiq/gobook/internal/cachekeys"
	appevents "github.com/scriptertoufiq/gobook/internal/events"
	"github.com/scriptertoufiq/gobook/pkg/cache"
	"github.com/scriptertoufiq/gobook/pkg/events"
)

// PostCacheListener keeps the post cache in step with the database.
//
// Writes go through the cache; deletes clear it:
//
//	created  write through — the new post is cached immediately
//	updated  write through — the cached copy is replaced with the new version
//	deleted  remove        — the key is dropped and stays gone
//
// Writing through on update means an edited post stays readable from cache: the
// value stored is the one just persisted, so no reader pays for a query to
// discover an edit that has already been made.
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

// onUpdated replaces the cached copy with the version that was just saved, so
// the next read is served the edit rather than having to go and find it.
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

	return l.forget(ctx, deleted.PostID)
}

// forget removes a post's cached copy and clears the listings it appeared in.
// Only deletes need this — a delete is the one case with no new value to store.
func (l *PostCacheListener) forget(ctx context.Context, id uint) error {
	if err := l.cache.Delete(ctx, cachekeys.Post(id)); err != nil {
		return fmt.Errorf("removing post %d from cache: %w", id, err)
	}

	return l.clearListings(ctx)
}

// store writes the post through to the cache, then clears the listings — a new
// or edited post changes what a page of results contains, and those pages are
// keyed by page number, search term and sort, so they cannot be rewritten
// individually the way a single post can.
func (l *PostCacheListener) store(ctx context.Context, id uint, value any) error {
	if err := l.cache.Set(ctx, cachekeys.Post(id), value, l.ttl); err != nil {
		return fmt.Errorf("caching post %d: %w", id, err)
	}

	return l.clearListings(ctx)
}

// clearListings invalidates every cached page of the post listing.
//
// A single INCR, not a scan. The previous version deleted by prefix, which
// visits the whole keyspace on every post written — measured at 828 requests a
// second against an empty Redis and 15 once unrelated keys had accumulated,
// for a prefix that matched nothing at all. Bumping a generation costs one
// command and does not care how large the cache is.
//
// Nothing caches listings yet. The counter is maintained anyway so that adding
// listing caching is a read-side change only: embed the generation in the key
// and correctness follows, with no chance of forgetting the invalidation.
func (l *PostCacheListener) clearListings(ctx context.Context) error {
	if _, err := l.cache.Bump(ctx, cachekeys.PostListGeneration()); err != nil {
		return fmt.Errorf("invalidating cached post listings: %w", err)
	}
	return nil
}
