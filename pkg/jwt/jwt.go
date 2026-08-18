// Package jwt wraps golang-jwt so the rest of the app never touches signing
// methods or claim maps directly.
package jwt

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Claims is what a validated access token carries. Role and Verified are
// embedded so authorisation middleware needs no database round trip.
type Claims struct {
	jwtlib.RegisteredClaims
	Role     string `json:"role"`
	Verified bool   `json:"verified"`
}

// UserID parses the subject back into the numeric id it was issued from.
func (c *Claims) UserID() (uint, error) {
	id, err := strconv.ParseUint(c.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("jwt: subject %q is not a user id: %w", c.Subject, err)
	}
	return uint(id), nil
}

// Manager issues and validates access tokens.
type Manager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func NewManager(secret, issuer string, accessTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), issuer: issuer, accessTTL: accessTTL}
}

// AccessTTL exposes the lifetime so callers can report `expires_in`.
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// Issue signs an access token for a user. The returned time is when it expires.
func (m *Manager) Issue(userID uint, role string, verified bool) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTTL)

	claims := Claims{
		RegisteredClaims: jwtlib.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			Issuer:    m.issuer,
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(expiresAt),
		},
		Role:     role,
		Verified: verified,
	}

	signed, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("jwt: sign: %w", err)
	}
	return signed, expiresAt, nil
}

// ErrInvalidToken covers every rejection reason. Callers must not distinguish
// between them at the HTTP layer — "expired" vs "bad signature" is information
// an attacker can probe with.
var ErrInvalidToken = errors.New("jwt: invalid token")

// Parse validates a token and returns its claims.
func (m *Manager) Parse(raw string) (*Claims, error) {
	var claims Claims

	_, err := jwtlib.ParseWithClaims(raw, &claims,
		func(t *jwtlib.Token) (any, error) {
			// Pin the algorithm. Without this check a token forged with
			// "alg":"none" — or an RS256 token using our public key as the HMAC
			// secret — would be accepted.
			if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
			}
			return m.secret, nil
		},
		jwtlib.WithIssuer(m.issuer),
		jwtlib.WithExpirationRequired(),
		jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	return &claims, nil
}
