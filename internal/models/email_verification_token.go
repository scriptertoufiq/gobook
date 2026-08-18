package models

import "time"

// EmailVerificationToken backs the link sent to a new user. Same storage rule
// as RefreshToken: hash only, never the plaintext.
type EmailVerificationToken struct {
	Base
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`

	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

func (EmailVerificationToken) TableName() string { return "email_verification_tokens" }

// IsUsable reports whether the token can still be redeemed — single use, and
// only before it expires.
func (t EmailVerificationToken) IsUsable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}
