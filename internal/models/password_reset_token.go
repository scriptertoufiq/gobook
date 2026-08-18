package models

import "time"

// PasswordResetToken backs the "forgot password" link. Same storage rule as the
// other token tables: the SHA-256 hash only, never the plaintext.
//
// These are shorter-lived than verification tokens — a reset link is a
// temporary key to an account, so the window to abuse a leaked one stays small.
type PasswordResetToken struct {
	Base
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`

	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

func (PasswordResetToken) TableName() string { return "password_reset_tokens" }

// IsUsable reports whether the token can still be redeemed — single use, and
// only before it expires.
func (t PasswordResetToken) IsUsable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}
