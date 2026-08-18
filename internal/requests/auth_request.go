package requests

// RegisterRequest deliberately has no Role field — a registrant must not be
// able to make themselves an admin. The service hardcodes models.RoleUser.
type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=120"`
	Email    string `json:"email" binding:"required,email,max=191"`
	Password string `json:"password" binding:"required,min=8,max=64"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest requires the caller to say what they mean: either a specific
// refresh_token to revoke, or "all": true. An empty body used to silently mean
// "everywhere", which is a surprising amount of blast radius to infer from
// absence.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required_without=All"`
	All          bool   `json:"all"`
}

// ChangePasswordRequest is the signed-in equivalent of a reset. The current
// password is always required: an access token is short-lived proof of a past
// login, not proof that the holder knows the credential.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	Password        string `json:"password" binding:"required,min=8,max=64"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest carries the emailed token plus the replacement password.
// The min=8 rule is the same one registration enforces — a reset must not be a
// back door to a weaker password.
type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=64"`
}
