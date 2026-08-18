// Package requests holds the input DTOs. Every write endpoint binds into one
// of these instead of into a model — that keeps clients from mass-assigning
// columns like `role` or `id` they shouldn't control.
package requests

type CreateUserRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=120"`
	Email    string `json:"email" binding:"required,email,max=191"`
	Password string `json:"password" binding:"required,min=8,max=64"`
	Role     string `json:"role" binding:"omitempty,oneof=user admin"`
}

// UpdateUserRequest uses pointers so "field absent" and "field set to zero"
// are distinguishable — a plain bool could never express "leave is_active alone".
type UpdateUserRequest struct {
	Name     *string `json:"name" binding:"omitempty,min=2,max=120"`
	Email    *string `json:"email" binding:"omitempty,email,max=191"`
	Password *string `json:"password" binding:"omitempty,min=8,max=64"`

	// CurrentPassword is required when a user changes their own password —
	// a stolen access token should not be enough to seize the account outright.
	// Admins acting on other accounts do not supply it.
	CurrentPassword *string `json:"current_password" binding:"omitempty"`
	Role            *string `json:"role" binding:"omitempty,oneof=user admin"`
	IsActive        *bool   `json:"is_active"`
}
