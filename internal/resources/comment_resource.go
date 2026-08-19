package resources

import (
	"time"

	"github.com/scriptertoufiq/gobook/internal/models"
)

// CommentResource is the API shape of a comment.
type CommentResource struct {
	ID       uint  `json:"id"`
	PostID   uint  `json:"post_id"`
	UserID   uint  `json:"user_id"`
	ParentID *uint `json:"parent_id"`

	Body string `json:"body"`

	// ReplyCount lets a client offer "view 3 replies" without asking again per
	// comment. Always zero on a reply, which cannot have replies of its own.
	ReplyCount int64 `json:"reply_count"`

	// Edited saves every client from comparing two timestamps to work out
	// whether to show the marker.
	Edited bool `json:"edited"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewCommentResource(c *models.Comment, replyCount int64) CommentResource {
	return CommentResource{
		ID:         c.ID,
		PostID:     c.PostID,
		UserID:     c.UserID,
		ParentID:   c.ParentID,
		Body:       c.Body,
		ReplyCount: replyCount,
		Edited:     c.UpdatedAt.After(c.CreatedAt),
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

// NewCommentCollection shapes a page of comments, taking each reply count from
// the map the service assembled in one query.
func NewCommentCollection(comments []models.Comment, replyCounts map[uint]int64) []CommentResource {
	out := make([]CommentResource, 0, len(comments))
	for i := range comments {
		out = append(out, NewCommentResource(&comments[i], replyCounts[comments[i].ID]))
	}
	return out
}
