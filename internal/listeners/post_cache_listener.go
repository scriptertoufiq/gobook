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
// The policy differs by event, because the events differ in what can go wrong:
//
//	created  write through — the post is cached immediately
//	updated  invalidate    — the key is dropped, next read repopulates
//	deleted  invalidate    — the key is dropped and stays gone
//
// Updates invalidate rather than overwrite because writing the new value back
// races any concurrent update: two edits landing together can leave the cache
// holding whichever reached Redis second, even though the database kept the
// other. Dropping the key cannot be wrong — the next read is authoritative by
// construction. The cost is one database query after each edit.
//
// Creates still write through: the row has only just come into existence, so
// there is no other writer to race with, and it spares the first reader a
// query for a post that was almost certainly created to be read.
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

// onUpdated drops the cached copy rather than replacing it. See the type's
// doc comment for why an edit invalidates while a create writes through.
func (l *PostCacheListener) onUpdated(ctx context.Context, event events.Event) error {
	updated, ok := event.(appevents.PostUpdated)
	if !ok {
		return fmt.Errorf("post-cache: expected PostUpdated, got %T", event)
	}
	if updated.Post == nil {
		return fmt.Errorf("post-cache: PostUpdated carried no post")
	}

	return l.forget(ctx, updated.Post.ID)
}

func (l *PostCacheListener) onDeleted(ctx context.Context, event events.Event) error {
	deleted, ok := event.(appevents.PostDeleted)
	if !ok {
		return fmt.Errorf("post-cache: expected PostDeleted, got %T", event)
	}

	return l.forget(ctx, deleted.PostID)
}

// forget removes a post's cached copy and clears the listings it appeared in.
// Shared by the update and delete paths, which want exactly the same thing.
func (l *PostCacheListener) forget(ctx context.Context, id uint) error {
	if err := l.cache.Delete(ctx, cachekeys.Post(id)); err != nil {
		return fmt.Errorf("removing post %d from cache: %w", id, err)
	}

	return l.clearListings(ctx)
}

// store writes the post through to the cache, then clears the listings — a new
// post changes what a page of results contains, and those pages are keyed by
// page number, search term and sort, so they cannot be updated individually.
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
