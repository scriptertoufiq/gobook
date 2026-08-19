package resources

import (
	"time"

	"github.com/scriptertoufiq/gobook/internal/models"
)

// PostResource is the API shape of a post. Going through an explicit struct
// means a column added to the model can never leak by accident.
type PostResource struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Reactions is always present, even as an empty tally, so a client never
	// has to handle a missing key.
	Reactions ReactionResource `json:"reactions"`

	// CommentCount is the whole conversation under this post, replies included.
	CommentCount int64 `json:"comment_count"`
}

func NewPostResource(p *models.Post) PostResource {
	return PostResource{
		ID:        p.ID,
		UserID:    p.UserID,
		Title:     p.Title,
		Content:   p.Content,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
		Reactions: EmptyReactions(),
	}
}

// WithReactions attaches a tally to a post resource.
func (r PostResource) WithReactions(reactions ReactionResource) PostResource {
	r.Reactions = reactions
	return r
}

// WithCommentCount attaches how much conversation a post has.
func (r PostResource) WithCommentCount(n int64) PostResource {
	r.CommentCount = n
	return r
}

func NewPostCollection(posts []models.Post) []PostResource {
	out := make([]PostResource, 0, len(posts))
	for i := range posts {
		out = append(out, NewPostResource(&posts[i]))
	}
	return out
}
