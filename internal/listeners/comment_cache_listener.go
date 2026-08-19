package listeners

import (
	"context"
	"fmt"

	"github.com/scriptertoufiq/gobook/internal/cachekeys"
	appevents "github.com/scriptertoufiq/gobook/internal/events"
	"github.com/scriptertoufiq/gobook/pkg/cache"
	"github.com/scriptertoufiq/gobook/pkg/events"
)

// CommentCacheListener invalidates a post's cached conversation whenever it
// changes.
//
// It bumps a counter rather than deleting anything. A post's comments are
// cached a page at a time, and the pages that exist are not individually known
// — page size, sort order and any search term are all part of the key. Hunting
// for them would mean scanning the keyspace on every comment written, which is
// the mistake this codebase already measured once: 828 requests a second down
// to 15 as unrelated keys accumulated.
//
// Raising the generation orphans every page for that post at once, at the cost
// of one command. The orphans are reclaimed by their TTL, which is why the
// comment cache must always have one.
type CommentCacheListener struct {
	cache cache.Cache
}

func NewCommentCacheListener(store cache.Cache) *CommentCacheListener {
	return &CommentCacheListener{cache: store}
}

// Register subscribes to all three verbs. They are handled identically — any
// change makes the cached pages untrustworthy — but they are registered
// separately so the reason a page was dropped stays visible in a log.
func (l *CommentCacheListener) Register(d *events.Dispatcher) {
	for _, name := range []string{
		appevents.CommentCreatedName,
		appevents.CommentUpdatedName,
		appevents.CommentDeletedName,
	} {
		d.Listen(name, "comment-cache", l.onChanged)
	}
}

func (l *CommentCacheListener) onChanged(ctx context.Context, event events.Event) error {
	changed, ok := event.(appevents.CommentChanged)
	if !ok {
		return fmt.Errorf("comment-cache: expected CommentChanged, got %T", event)
	}

	if _, err := l.cache.Bump(ctx, cachekeys.CommentGeneration(changed.PostID)); err != nil {
		return fmt.Errorf("invalidating cached comments for post %d: %w", changed.PostID, err)
	}

	// The pages are versioned, but a single cached comment is not — it is
	// dropped by id, which the event carries precisely so this can be exact
	// rather than another sweep.
	if changed.CommentID != 0 {
		if err := l.cache.Delete(ctx, cachekeys.Comment(changed.CommentID)); err != nil {
			return fmt.Errorf("invalidating cached comment %d: %w", changed.CommentID, err)
		}
	}

	return nil
}
