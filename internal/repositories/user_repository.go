package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/pkg/pagination"
)

// UserRepository is the contract the service depends on. Keeping it an
// interface means the service can be unit-tested against a fake, and swapping
// GORM out later touches only this package.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	EmailTaken(ctx context.Context, email string, excludeID uint) (bool, error)
	Paginate(ctx context.Context, p pagination.Params) ([]models.User, int64, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// userSortable whitelists the columns clients may sort by.
var userSortable = []string{"id", "name", "email", "created_at"}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.User{}, id).Error
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// EmailTaken deliberately runs Unscoped. A soft-deleted user keeps its row, and
// that row keeps occupying the unique index on `email` — so the default scope,
// which hides deleted rows, would report an address as free and let the INSERT
// fail with a raw constraint violation (a 500) instead of a clean 409.
func (r *userRepository) EmailTaken(ctx context.Context, email string, excludeID uint) (bool, error) {
	query := r.db.WithContext(ctx).Unscoped().Model(&models.User{}).Where("email = ?", email)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *userRepository) Paginate(ctx context.Context, p pagination.Params) ([]models.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.User{})

	if p.Search != "" {
		like := "%" + p.Search + "%"
		query = query.Where("name LIKE ? OR email LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []models.User
	err := query.
		Order(p.OrderClause(userSortable, "id")).
		Limit(p.PerPage).
		Offset(p.Offset()).
		Find(&users).Error

	return users, total, err
}
