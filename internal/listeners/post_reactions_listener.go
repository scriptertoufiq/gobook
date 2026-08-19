package listeners

import (
	"context"
	"fmt"

	appevents "github.com/scriptertoufiq/go-mvc/internal/events"
	"github.com/scriptertoufiq/go-mvc/internal/reactions"
	"github.com/scriptertoufiq/go-mvc/pkg/events"
)

// PostReactionsListener clears a deleted post's tally.
//
// Reaction keys deliberately carry no expiry — an unflushed reaction that
// Redis discards is simply lost — which means nothing reclaims them on its
// own. The rows go with the post through the foreign key, so without this the
// cached tally would outlive the post it counts, forever.
type PostReactionsListener struct {
	store *reactions.Store
}

func NewPostReactionsListener(store *reactions.Store) *PostReactionsListener {
	return &PostReactionsListener{store: store}
}

func (l *PostReactionsListener) Register(d *events.Dispatcher) {
	d.Listen(appevents.PostDeletedName, "post-reactions", l.onPostDeleted)
}

func (l *PostReactionsListener) onPostDeleted(ctx context.Context, event events.Event) error {
	deleted, ok := event.(appevents.PostDeleted)
	if !ok {
		return fmt.Errorf("post-reactions: expected PostDeleted, got %T", event)
	}

	// Only the tally and its hydrated marker are cleared. The per-user keys are
	// left alone deliberately: a viral post could have millions, and scanning
	// for them would stall Redis at exactly the wrong moment. Nothing reads a
	// deleted post, and any pending ones still flush — where the foreign key
	// drops them.
	return l.store.Forget(ctx, deleted.PostID)
}
