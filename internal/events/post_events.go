// Package events holds the application's event types. The bus itself lives in
// pkg/events; these are the messages that travel on it.
package events

import "github.com/scriptertoufiq/gobook/internal/models"

// Event names. Constants rather than literals so a typo is a compile error
// instead of a listener that silently never fires.
const (
	PostCreatedName = "post.created"
	PostUpdatedName = "post.updated"
	PostDeletedName = "post.deleted"

	CommentCreatedName = "comment.created"
	CommentUpdatedName = "comment.updated"
	CommentDeletedName = "comment.deleted"
)

// CommentChanged is emitted whenever a post's conversation changes.
//
// One event type for all three verbs, because every listener so far reacts the
// same way: the cached pages of that post's conversation are no longer
// trustworthy. The verb is carried for anything that later needs to tell them
// apart — a notification, say — without needing three near-identical types now.
type CommentChanged struct {
	PostID    uint
	CommentID uint
	// Verb is one of the Comment*Name constants.
	Verb string
}

func (c CommentChanged) Name() string { return c.Verb }

// PostCreated is emitted after a post is persisted.
type PostCreated struct {
	Post *models.Post
}

func (PostCreated) Name() string { return PostCreatedName }

// PostUpdated is emitted after a post is saved, and carries the post as it now
// exists in the database.
type PostUpdated struct {
	Post *models.Post
}

func (PostUpdated) Name() string { return PostUpdatedName }

// PostDeleted carries only the id: the row is gone, so there is nothing else
// a listener could trust.
type PostDeleted struct {
	PostID uint
	// AuthorID is kept because listeners that maintain per-author views need
	// to know whose lists to clear, and cannot look it up any more.
	AuthorID uint
}

func (PostDeleted) Name() string { return PostDeletedName }
