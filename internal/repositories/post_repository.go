package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
	"github.com/scriptertoufiq/go-mvc/pkg/pagination"
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
	Paginate(ctx context.Context, p pagination.Params, userID uint) ([]models.Post, int64, error)
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
) ([]models.Post, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Post{})

	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if p.Search != "" {
		like := "%" + p.Search + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var posts []models.Post
	err := query.
		Order(p.OrderClause(postSortable, "id")).
		Limit(p.PerPage).
		Offset(p.Offset()).
		Find(&posts).Error

	return posts, total, err
}
