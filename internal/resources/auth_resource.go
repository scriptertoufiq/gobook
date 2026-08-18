package resources

import (
	"time"

	"github.com/scriptertoufiq/go-mvc/internal/services"
)

// TokenResource is the login/refresh payload. expires_in is included alongside
// expires_at because most HTTP clients find a duration easier to schedule a
// refresh against than an absolute timestamp.
type TokenResource struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresAt    time.Time    `json:"expires_at"`
	ExpiresIn    int          `json:"expires_in"`
	User         UserResource `json:"user"`
}

func NewTokenResource(pair *services.TokenPair) TokenResource {
	return TokenResource{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    pair.TokenType,
		ExpiresAt:    pair.ExpiresAt,
		ExpiresIn:    int(time.Until(pair.ExpiresAt).Seconds()),
		User:         NewUserResource(pair.User),
	}
}
