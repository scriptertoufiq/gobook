package models

import "time"

// User is both the GORM schema and the domain entity.
type User struct {
	Base
	Name     string `gorm:"size:120;not null" json:"name"`
	Email    string `gorm:"size:191;not null;uniqueIndex" json:"email"`
	Password string `gorm:"size:255;not null" json:"-"` // never serialised
	Role     string `gorm:"size:32;not null;default:user" json:"role"`
	IsActive bool   `gorm:"not null;default:true" json:"is_active"`

	// EmailVerifiedAt is nil until the user follows the verification link.
	// Nullable-timestamp-as-flag keeps *when* it happened, which a bool loses.
	EmailVerifiedAt *time.Time `gorm:"index" json:"email_verified_at,omitempty"`

	Timestamps
}

// TableName pins the table name so refactors of the Go type can't rename it.
func (User) TableName() string { return "users" }

func (u User) IsVerified() bool { return u.EmailVerifiedAt != nil }

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)
