package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
)

type PasswordResetRepository interface {
	Create(ctx context.Context, token *models.PasswordResetToken) error
	FindByHash(ctx context.Context, hash string) (*models.PasswordResetToken, error)
	// MarkUsed reports whether this call was the one that consumed the token.
	MarkUsed(ctx context.Context, id uint, at time.Time) (bool, error)
	InvalidateForUser(ctx context.Context, userID uint, at time.Time) error
}

type passwordResetRepository struct {
	db *gorm.DB
}

func NewPasswordResetRepository(db *gorm.DB) PasswordResetRepository {
	return &passwordResetRepository{db: db}
}

func (r *passwordResetRepository) Create(ctx context.Context, token *models.PasswordResetToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *passwordResetRepository) FindByHash(ctx context.Context, hash string) (*models.PasswordResetToken, error) {
	var token models.PasswordResetToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *passwordResetRepository) MarkUsed(ctx context.Context, id uint, at time.Time) (bool, error) {
	// Single-use is enforced here, not by an earlier read: the `used_at IS
	// NULL` predicate makes this UPDATE the critical section.
	result := r.db.WithContext(ctx).
		Model(&models.PasswordResetToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", at)

	return result.RowsAffected == 1, result.Error
}

// InvalidateForUser burns every outstanding link before a new one is issued, so
// requesting a second reset email silently retires the first.
func (r *passwordResetRepository) InvalidateForUser(ctx context.Context, userID uint, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", at).Error
}
