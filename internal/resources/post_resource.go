package resources

import (
	"time"

	"github.com/scriptertoufiq/go-mvc/internal/models"
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
}

func NewPostResource(p *models.Post) PostResource {
	return PostResource{
		ID:        p.ID,
		UserID:    p.UserID,
		Title:     p.Title,
		Content:   p.Content,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func NewPostCollection(posts []models.Post) []PostResource {
	out := make([]PostResource, 0, len(posts))
	for i := range posts {
		out = append(out, NewPostResource(&posts[i]))
	}
	return out
}
