package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	FindByHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	// Revoke reports whether this call was the one that revoked the token.
	// False means it was already spent — the caller lost a race.
	Revoke(ctx context.Context, id uint, at time.Time) (bool, error)
	RevokeAllForUser(ctx context.Context, userID uint, at time.Time) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

type refreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

func (r *refreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *refreshTokenRepository) FindByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// Revoke is the atomicity gate for rotation. The `revoked_at IS NULL` predicate
// makes the UPDATE itself the critical section, so exactly one of N concurrent
// callers can see RowsAffected == 1.
func (r *refreshTokenRepository) Revoke(ctx context.Context, id uint, at time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at)

	return result.RowsAffected == 1, result.Error
}

// RevokeAllForUser is "log out everywhere" — also the right response to a
// password change or a suspected compromise.
func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uint, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at).Error
}

// DeleteExpired hard-deletes rows that can no longer be presented. Nothing
// calls this on a schedule yet — wire it to a cron when the table grows.
func (r *refreshTokenRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Unscoped(). // genuinely remove them; a soft-deleted token has no value
		Where("expires_at < ?", before).
		Delete(&models.RefreshToken{})

	return result.RowsAffected, result.Error
}
