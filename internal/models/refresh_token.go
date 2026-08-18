package models

import "time"

// RefreshToken is a single issued refresh credential. Only the SHA-256 hash of
// the token is stored, so a database leak yields nothing presentable to the API.
//
// Rows are kept after use rather than deleted: a revoked row is what lets
// AuthService detect a replayed token instead of treating it as merely unknown.
type RefreshToken struct {
	Base
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"` // sha256 hex = 64 chars
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`

	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`

	Timestamps
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

// IsUsable reports whether the token can still be exchanged.
func (t RefreshToken) IsUsable(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}
