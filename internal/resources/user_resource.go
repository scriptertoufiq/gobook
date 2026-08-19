// Package resources shapes models into API output. Going through an explicit
// struct means adding a column to a model can never accidentally leak it.
package resources

import (
	"time"

	"github.com/scriptertoufiq/gobook/internal/models"
)

type UserResource struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	IsActive        bool       `json:"is_active"`
	EmailVerified   bool       `json:"email_verified"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func NewUserResource(u *models.User) UserResource {
	return UserResource{
		ID:              u.ID,
		Name:            u.Name,
		Email:           u.Email,
		Role:            u.Role,
		IsActive:        u.IsActive,
		EmailVerified:   u.IsVerified(),
		EmailVerifiedAt: u.EmailVerifiedAt,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
	}
}

func NewUserCollection(users []models.User) []UserResource {
	out := make([]UserResource, 0, len(users))
	for i := range users {
		out = append(out, NewUserResource(&users[i]))
	}
	return out
}
