package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/pkg/pagination"
)

// PostRepository is the contract the service depends on. Same reasoning as
// UserRepository: an interface here is what lets the service be unit-tested
// against a fake, and confines GORM to this package.
type PostRepository interface {
	Create(ctx context.Context, post *models.Post) error
	Update(ctx context.Context, post *models.Post) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*models.Post, error)
	// Paginate lists posts, optionally narrowed to one author. Pass 0 for all.
	//
	// Reports whether another page exists rather than how many rows match.
	// The feed is read forwards and never shows a total, and counting two
	// million rows to answer a question nobody asked costs more than the page.
	Paginate(ctx context.Context, p pagination.Params, userID uint) ([]models.Post, bool, error)
}

type postRepository struct {
	db *gorm.DB
}

func NewPostRepository(db *gorm.DB) PostRepository {
	return &postRepository{db: db}
}

// postSortable whitelists the columns clients may sort by. Anything else falls
// back to the default, so a query parameter never reaches the SQL builder.
var postSortable = []string{"id", "title", "created_at", "updated_at"}

func (r *postRepository) Create(ctx context.Context, post *models.Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *postRepository) Update(ctx context.Context, post *models.Post) error {
	return r.db.WithContext(ctx).Save(post).Error
}

func (r *postRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Post{}, id).Error
}

func (r *postRepository) FindByID(ctx context.Context, id uint) (*models.Post, error) {
	var post models.Post
	if err := r.db.WithContext(ctx).First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *postRepository) Paginate(
	ctx context.Context,
	p pagination.Params,
	userID uint,
) ([]models.Post, bool, error) {
	query := r.db.WithContext(ctx).Model(&models.Post{})

	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if p.Search != "" {
		like := "%" + p.Search + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}

	// One row more than the page needs. Its presence is the whole answer to
	// "is there another page", and it never reaches the caller.
	var posts []models.Post
	err := query.
		Order(p.OrderClause(postSortable, "id")).
		Limit(p.Lookahead()).
		Offset(p.Offset()).
		Find(&posts).Error
	if err != nil {
		return nil, false, err
	}

	hasMore := len(posts) > p.PerPage
	if hasMore {
		posts = posts[:p.PerPage]
	}

	return posts, hasMore, nil
}
