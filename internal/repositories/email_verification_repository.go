package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
)

type EmailVerificationRepository interface {
	Create(ctx context.Context, token *models.EmailVerificationToken) error
	FindByHash(ctx context.Context, hash string) (*models.EmailVerificationToken, error)
	// MarkUsed reports whether this call was the one that consumed the token.
	MarkUsed(ctx context.Context, id uint, at time.Time) (bool, error)
	InvalidateForUser(ctx context.Context, userID uint, at time.Time) error
}

type emailVerificationRepository struct {
	db *gorm.DB
}

func NewEmailVerificationRepository(db *gorm.DB) EmailVerificationRepository {
	return &emailVerificationRepository{db: db}
}

func (r *emailVerificationRepository) Create(ctx context.Context, token *models.EmailVerificationToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *emailVerificationRepository) FindByHash(ctx context.Context, hash string) (*models.EmailVerificationToken, error) {
	var token models.EmailVerificationToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *emailVerificationRepository) MarkUsed(ctx context.Context, id uint, at time.Time) (bool, error) {
	// Single-use is enforced here, not by an earlier read: the `used_at IS
	// NULL` predicate makes this UPDATE the critical section.
	result := r.db.WithContext(ctx).
		Model(&models.EmailVerificationToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", at)

	return result.RowsAffected == 1, result.Error
}

// InvalidateForUser burns any outstanding links before a new one is issued, so
// a resend leaves exactly one working token rather than a growing pile.
func (r *emailVerificationRepository) InvalidateForUser(ctx context.Context, userID uint, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.EmailVerificationToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", at).Error
}
