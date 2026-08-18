package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
	"github.com/scriptertoufiq/go-mvc/internal/repositories"
	"github.com/scriptertoufiq/go-mvc/internal/requests"
	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/jwt"
	"github.com/scriptertoufiq/go-mvc/pkg/mailer"
	"github.com/scriptertoufiq/go-mvc/pkg/token"
)

// AuthOptions carries the knobs from config so the service never imports it.
type AuthOptions struct {
	AppName                  string
	AppURL                   string
	RequireEmailVerification bool
	VerificationTTL          time.Duration
	RefreshTTL               time.Duration
	PasswordResetTTL         time.Duration
	PasswordResetURL         string

	// Dispatcher runs work that must not block the response. Nil means `go f()`.
	// Tests substitute an inline runner so assertions stay deterministic — the
	// same seam ratelimit.SetClock provides.
	Dispatcher func(func())
}

// AuthService owns registration, login, token lifecycle and email verification.
// Like every other service it knows nothing about HTTP.
type AuthService struct {
	users          *UserService
	refreshTokens  repositories.RefreshTokenRepository
	verifications  repositories.EmailVerificationRepository
	passwordResets repositories.PasswordResetRepository
	userRepo       repositories.UserRepository
	jwt            *jwt.Manager
	mailer         mailer.Mailer
	opts           AuthOptions
}

func NewAuthService(
	users *UserService,
	userRepo repositories.UserRepository,
	refreshTokens repositories.RefreshTokenRepository,
	verifications repositories.EmailVerificationRepository,
	passwordResets repositories.PasswordResetRepository,
	jwtManager *jwt.Manager,
	mail mailer.Mailer,
	opts AuthOptions,
) *AuthService {
	return &AuthService{
		users:          users,
		userRepo:       userRepo,
		refreshTokens:  refreshTokens,
		verifications:  verifications,
		passwordResets: passwordResets,
		jwt:            jwtManager,
		mailer:         mail,
		opts:           opts,
	}
}

// background runs f detached from the request. The request context is not
// reused: it is cancelled the moment the handler returns, which would abort the
// very work we deferred.
func (s *AuthService) background(f func(context.Context)) {
	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		f(ctx)
	}

	if s.opts.Dispatcher != nil {
		s.opts.Dispatcher(run)
		return
	}
	go run()
}

// TokenPair is what a successful login or refresh hands back.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
	User         *models.User
}

// errInvalidCredentials is returned for every refresh-token rejection —
// unknown, expired, revoked, or belonging to a deleted user. Distinguishing
// them would let an attacker probe which tokens ever existed.
func errInvalidRefresh() *apperror.Error {
	return apperror.Unauthorized("Invalid or expired refresh token.")
}

// Register creates the account and, when verification is enabled, emails a link.
func (s *AuthService) Register(ctx context.Context, req requests.RegisterRequest) (*models.User, error) {
	user, err := s.users.Create(ctx, requests.CreateUserRequest{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     models.RoleUser, // never let a registrant choose their own role
	})
	if err != nil {
		return nil, err
	}

	// The verification email is sent by UserService.Create via the
	// needs-verification hook, so every creation path behaves identically.
	return user, nil
}

// Login authenticates and issues a token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	user, err := s.users.Authenticate(ctx, email, password)
	if err != nil {
		return nil, err
	}
	return s.issuePair(ctx, user)
}

// Refresh exchanges a refresh token for a new pair and revokes the old one.
// Rotation means a stolen token is usable at most once, and the theft surfaces
// when the legitimate holder's next refresh fails.
func (s *AuthService) Refresh(ctx context.Context, plaintext string) (*TokenPair, error) {
	if plaintext == "" {
		return nil, errInvalidRefresh()
	}

	stored, err := s.refreshTokens.FindByHash(ctx, token.Hash(plaintext))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errInvalidRefresh()
		}
		return nil, apperror.Internal(err)
	}

	now := time.Now()
	if !stored.IsUsable(now) {
		return nil, errInvalidRefresh()
	}

	user, err := s.userRepo.FindByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errInvalidRefresh()
		}
		return nil, apperror.Internal(err)
	}
	if !user.IsActive {
		return nil, apperror.Forbidden("This account is disabled.")
	}

	// Revoking is the critical section, not the IsUsable check above. Two
	// concurrent refreshes both pass that check; only one can win this UPDATE,
	// and the loser must be rejected — otherwise a stolen token is spendable
	// more than once, which is the whole property rotation exists to provide.
	won, err := s.refreshTokens.Revoke(ctx, stored.ID, now)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !won {
		return nil, errInvalidRefresh()
	}

	return s.issuePair(ctx, user)
}

// Logout revokes a single refresh token belonging to the caller.
//
// It is deliberately idempotent and silent: an unknown token, or one belonging
// to somebody else, still reports success. Saying "that token isn't yours"
// would confirm it exists, and refusing loudly helps no legitimate client —
// but revoking it would let anyone who scraped a token end another user's
// session, so ownership is checked before acting.
func (s *AuthService) Logout(ctx context.Context, userID uint, plaintext string) error {
	if plaintext == "" {
		return nil
	}

	stored, err := s.refreshTokens.FindByHash(ctx, token.Hash(plaintext))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return apperror.Internal(err)
	}

	if stored.UserID != userID {
		return nil
	}

	if _, err := s.refreshTokens.Revoke(ctx, stored.ID, time.Now()); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// LogoutAll revokes every refresh token a user holds.
func (s *AuthService) LogoutAll(ctx context.Context, userID uint) error {
	if err := s.refreshTokens.RevokeAllForUser(ctx, userID, time.Now()); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// VerifyEmail redeems a verification token and stamps the user.
func (s *AuthService) VerifyEmail(ctx context.Context, plaintext string) (*models.User, error) {
	invalid := apperror.BadRequest("This verification link is invalid or has expired.")
	if plaintext == "" {
		return nil, invalid
	}

	stored, err := s.verifications.FindByHash(ctx, token.Hash(plaintext))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, invalid
		}
		return nil, apperror.Internal(err)
	}

	now := time.Now()

	user, err := s.userRepo.FindByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, invalid
		}
		return nil, apperror.Internal(err)
	}

	// Check the user's state *before* the token's. Redeeming marks the token
	// used, so an already-verified user clicking their link a second time —
	// ordinary human behaviour — would otherwise be told it is invalid. The
	// outcome they wanted has already happened, so report success.
	if user.IsVerified() {
		return user, nil
	}

	// Unverified user with an unusable token: expired, or superseded by a
	// resend. Both are genuinely invalid.
	if !stored.IsUsable(now) {
		return nil, invalid
	}

	// Consume the token before acting on it, so concurrent redemptions of the
	// same link cannot both proceed.
	consumed, err := s.verifications.MarkUsed(ctx, stored.ID, now)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !consumed {
		return nil, invalid
	}

	user.EmailVerifiedAt = &now
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, apperror.Internal(err)
	}

	return user, nil
}

// ResendVerification issues a fresh link, invalidating any outstanding one.
func (s *AuthService) ResendVerification(ctx context.Context, userID uint) error {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsVerified() {
		return apperror.Conflict("This email address is already verified.")
	}

	if err := s.sendVerification(ctx, user); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// ForgotPassword emails a reset link — and always reports success.
//
// Every branch below returns nil: unknown address, disabled account, even a
// mail-server outage. Any observable difference would turn this endpoint into
// an account-enumeration oracle, and it is unauthenticated, so anyone could
// walk a list of addresses through it. Failures are logged for the operator
// instead of surfaced to the caller.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.userRepo.FindByEmail(ctx, normaliseEmail(email))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		// A genuine database fault is ours, not the caller's — but reporting it
		// would still only happen for addresses that exist. Stay quiet.
		log.Printf("auth: password reset lookup failed for %q: %v", email, err)
		return nil
	}

	if !user.IsActive {
		log.Printf("auth: password reset requested for disabled account %s", user.Email)
		return nil
	}

	// Dispatched, not awaited — and this is a security property, not a latency
	// tweak. Minting a token, writing a row and reaching SMTP takes measurably
	// longer than the early return taken for an unknown address, so doing it
	// inline would leak account existence through the clock even though every
	// response body is identical.
	s.background(func(bg context.Context) {
		if err := s.sendPasswordReset(bg, user); err != nil {
			log.Printf("auth: could not send password reset email to %s: %v", user.Email, err)
		}
	})

	return nil
}

// ResetPassword redeems a reset token and installs the new password.
//
// Every active session is destroyed as part of this. Someone resetting a
// password is often doing so *because* the account is compromised; leaving the
// attacker's refresh tokens alive would defeat the whole exercise.
func (s *AuthService) ResetPassword(ctx context.Context, plaintext, newPassword string) error {
	invalid := apperror.BadRequest("This password reset link is invalid or has expired.")
	if plaintext == "" {
		return invalid
	}

	stored, err := s.passwordResets.FindByHash(ctx, token.Hash(plaintext))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalid
		}
		return apperror.Internal(err)
	}

	now := time.Now()
	if !stored.IsUsable(now) {
		return invalid
	}

	user, err := s.userRepo.FindByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalid
		}
		return apperror.Internal(err)
	}
	if !user.IsActive {
		return apperror.Forbidden("This account is disabled.")
	}

	// Consume the token first. If two requests race, only one may change the
	// password — otherwise the later one silently overwrites the earlier.
	consumed, err := s.passwordResets.MarkUsed(ctx, stored.ID, now)
	if err != nil {
		return apperror.Internal(err)
	}
	if !consumed {
		return invalid
	}

	// Reuse UserService.Update so hashing — and session revocation, which it
	// performs for any password change — stay in exactly one place.
	if _, err := s.users.Update(ctx, user.ID, requests.UpdateUserRequest{Password: &newPassword}); err != nil {
		return err
	}

	return nil
}

// sendPasswordReset mints a link and hands it to the mailer.
func (s *AuthService) sendPasswordReset(ctx context.Context, user *models.User) error {
	now := time.Now()
	if err := s.passwordResets.InvalidateForUser(ctx, user.ID, now); err != nil {
		return fmt.Errorf("invalidate previous reset tokens: %w", err)
	}

	plaintext, hashed, err := token.Generate()
	if err != nil {
		return err
	}

	record := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashed,
		ExpiresAt: now.Add(s.opts.PasswordResetTTL),
	}
	if err := s.passwordResets.Create(ctx, record); err != nil {
		return fmt.Errorf("store reset token: %w", err)
	}

	// Points at the frontend form, not the API — the user has to type a new
	// password, so a bare GET endpoint could not complete the flow.
	link := fmt.Sprintf("%s?token=%s", s.opts.PasswordResetURL, url.QueryEscape(plaintext))
	subject := fmt.Sprintf("Reset your %s password", s.opts.AppName)

	return s.mailer.Send(ctx, user.Email, subject, passwordResetEmailHTML(user.Name, link, s.opts.PasswordResetTTL))
}

func passwordResetEmailHTML(name, link string, ttl time.Duration) string {
	return fmt.Sprintf(`<!doctype html>
<html>
  <body style="font-family: system-ui, sans-serif; line-height: 1.6;">
    <p>Hi %s,</p>
    <p>We received a request to reset your password. Choose a new one here:</p>
    <p><a href="%s" style="display:inline-block;padding:10px 18px;background:#2563eb;color:#fff;text-decoration:none;border-radius:6px;">Reset password</a></p>
    <p>Or paste this link into your browser:<br><a href="%s">%s</a></p>
    <p style="color:#666;font-size:14px;">This link expires in %.0f minutes and can be used once. If you didn't request a reset, you can ignore this email — your password will not change.</p>
  </body>
</html>`, name, link, link, link, ttl.Minutes())
}

// HandleEmailNeedsVerification is registered on UserService and runs whenever
// an address needs proving — a new account, or a changed one.
func (s *AuthService) HandleEmailNeedsVerification(ctx context.Context, user *models.User) {
	if !s.opts.RequireEmailVerification {
		return
	}

	s.background(func(bg context.Context) {
		if err := s.sendVerification(bg, user); err != nil {
			log.Printf("auth: could not send verification email to %s: %v", user.Email, err)
		}
	})
}

// ChangePassword updates a signed-in user's own password and returns a fresh
// token pair.
//
// UserService.Update revokes every session on a password change — including the
// caller's — so without reissuing here, changing your password would sign you
// out of the device you are holding. Every other session stays revoked, which
// is the point.
func (s *AuthService) ChangePassword(ctx context.Context, userID uint, current, next string) (*TokenPair, error) {
	updated, err := s.users.Update(ctx, userID, requests.UpdateUserRequest{
		Password:        &next,
		CurrentPassword: &current,
	})
	if err != nil {
		return nil, err
	}

	return s.issuePair(ctx, updated)
}

// issuePair signs an access token and persists a matching refresh token.
func (s *AuthService) issuePair(ctx context.Context, user *models.User) (*TokenPair, error) {
	access, expiresAt, err := s.jwt.Issue(user.ID, user.Role, user.IsVerified())
	if err != nil {
		return nil, apperror.Internal(err)
	}

	plaintext, hashed, err := token.Generate()
	if err != nil {
		return nil, apperror.Internal(err)
	}

	record := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashed,
		ExpiresAt: time.Now().Add(s.opts.RefreshTTL),
	}
	if err := s.refreshTokens.Create(ctx, record); err != nil {
		return nil, apperror.Internal(err)
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: plaintext,
		TokenType:    "Bearer",
		ExpiresAt:    expiresAt,
		User:         user,
	}, nil
}

// sendVerification mints a link and hands it to the mailer.
func (s *AuthService) sendVerification(ctx context.Context, user *models.User) error {
	now := time.Now()
	if err := s.verifications.InvalidateForUser(ctx, user.ID, now); err != nil {
		return fmt.Errorf("invalidate previous verification tokens: %w", err)
	}

	plaintext, hashed, err := token.Generate()
	if err != nil {
		return err
	}

	record := &models.EmailVerificationToken{
		UserID:    user.ID,
		TokenHash: hashed,
		ExpiresAt: now.Add(s.opts.VerificationTTL),
	}
	if err := s.verifications.Create(ctx, record); err != nil {
		return fmt.Errorf("store verification token: %w", err)
	}

	link := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", s.opts.AppURL, url.QueryEscape(plaintext))
	subject := fmt.Sprintf("Verify your %s email address", s.opts.AppName)

	return s.mailer.Send(ctx, user.Email, subject, verificationEmailHTML(user.Name, link, s.opts.VerificationTTL))
}

func verificationEmailHTML(name, link string, ttl time.Duration) string {
	return fmt.Sprintf(`<!doctype html>
<html>
  <body style="font-family: system-ui, sans-serif; line-height: 1.6;">
    <p>Hi %s,</p>
    <p>Please confirm your email address to finish setting up your account.</p>
    <p><a href="%s" style="display:inline-block;padding:10px 18px;background:#2563eb;color:#fff;text-decoration:none;border-radius:6px;">Verify email address</a></p>
    <p>Or paste this link into your browser:<br><a href="%s">%s</a></p>
    <p style="color:#666;font-size:14px;">This link expires in %.0f hours. If you didn't create an account, you can ignore this email.</p>
  </body>
</html>`, name, link, link, link, ttl.Hours())
}
