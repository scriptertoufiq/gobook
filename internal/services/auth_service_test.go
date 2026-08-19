package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/internal/repositories"
	"github.com/scriptertoufiq/gobook/internal/requests"
	"github.com/scriptertoufiq/gobook/internal/services"
	"github.com/scriptertoufiq/gobook/pkg/jwt"
	"github.com/scriptertoufiq/gobook/pkg/mailer"
)

// ---------------------------------------------------------------- fake repos

type fakeRefreshRepo struct {
	tokens map[uint]*models.RefreshToken
	nextID uint
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{tokens: map[uint]*models.RefreshToken{}, nextID: 1}
}

var _ repositories.RefreshTokenRepository = (*fakeRefreshRepo)(nil)

func (f *fakeRefreshRepo) Create(_ context.Context, t *models.RefreshToken) error {
	t.ID = f.nextID
	f.nextID++
	f.tokens[t.ID] = t
	return nil
}

func (f *fakeRefreshRepo) FindByHash(_ context.Context, hash string) (*models.RefreshToken, error) {
	for _, t := range f.tokens {
		if t.TokenHash == hash {
			return t, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRefreshRepo) Revoke(_ context.Context, id uint, at time.Time) (bool, error) {
	if t, ok := f.tokens[id]; ok && t.RevokedAt == nil {
		t.RevokedAt = &at
		return true, nil
	}
	return false, nil // already revoked: this caller lost the race
}

func (f *fakeRefreshRepo) RevokeAllForUser(_ context.Context, userID uint, at time.Time) error {
	for _, t := range f.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &at
		}
	}
	return nil
}

func (f *fakeRefreshRepo) DeleteExpired(_ context.Context, before time.Time) (int64, error) {
	var n int64
	for id, t := range f.tokens {
		if t.ExpiresAt.Before(before) {
			delete(f.tokens, id)
			n++
		}
	}
	return n, nil
}

type fakeVerificationRepo struct {
	tokens map[uint]*models.EmailVerificationToken
	nextID uint
}

func newFakeVerificationRepo() *fakeVerificationRepo {
	return &fakeVerificationRepo{tokens: map[uint]*models.EmailVerificationToken{}, nextID: 1}
}

var _ repositories.EmailVerificationRepository = (*fakeVerificationRepo)(nil)

func (f *fakeVerificationRepo) Create(_ context.Context, t *models.EmailVerificationToken) error {
	t.ID = f.nextID
	f.nextID++
	f.tokens[t.ID] = t
	return nil
}

func (f *fakeVerificationRepo) FindByHash(_ context.Context, hash string) (*models.EmailVerificationToken, error) {
	for _, t := range f.tokens {
		if t.TokenHash == hash {
			return t, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeVerificationRepo) MarkUsed(_ context.Context, id uint, at time.Time) (bool, error) {
	if t, ok := f.tokens[id]; ok && t.UsedAt == nil {
		t.UsedAt = &at
		return true, nil
	}
	return false, nil
}

func (f *fakeVerificationRepo) InvalidateForUser(_ context.Context, userID uint, at time.Time) error {
	for _, t := range f.tokens {
		if t.UserID == userID && t.UsedAt == nil {
			t.UsedAt = &at
		}
	}
	return nil
}

type fakePasswordResetRepo struct {
	tokens map[uint]*models.PasswordResetToken
	nextID uint
}

func newFakePasswordResetRepo() *fakePasswordResetRepo {
	return &fakePasswordResetRepo{tokens: map[uint]*models.PasswordResetToken{}, nextID: 1}
}

var _ repositories.PasswordResetRepository = (*fakePasswordResetRepo)(nil)

func (f *fakePasswordResetRepo) Create(_ context.Context, t *models.PasswordResetToken) error {
	t.ID = f.nextID
	f.nextID++
	f.tokens[t.ID] = t
	return nil
}

func (f *fakePasswordResetRepo) FindByHash(_ context.Context, hash string) (*models.PasswordResetToken, error) {
	for _, t := range f.tokens {
		if t.TokenHash == hash {
			return t, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakePasswordResetRepo) MarkUsed(_ context.Context, id uint, at time.Time) (bool, error) {
	if t, ok := f.tokens[id]; ok && t.UsedAt == nil {
		t.UsedAt = &at
		return true, nil
	}
	return false, nil
}

func (f *fakePasswordResetRepo) InvalidateForUser(_ context.Context, userID uint, at time.Time) error {
	for _, t := range f.tokens {
		if t.UserID == userID && t.UsedAt == nil {
			t.UsedAt = &at
		}
	}
	return nil
}

// recordingMailer captures what would have been sent.
type recordingMailer struct {
	sent []sentMail
	err  error
}

type sentMail struct{ To, Subject, Body string }

var _ mailer.Mailer = (*recordingMailer)(nil)

func (m *recordingMailer) Send(_ context.Context, to, subject, body string) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, sentMail{To: to, Subject: subject, Body: body})
	return nil
}

// ------------------------------------------------------------------ harness

type authHarness struct {
	auth           *services.AuthService
	users          *fakeUserRepo // reused from user_service_test.go
	refreshTokens  *fakeRefreshRepo
	verifications  *fakeVerificationRepo
	passwordResets *fakePasswordResetRepo
	mail           *recordingMailer
	jwt            *jwt.Manager
	userService    *services.UserService
}

func newAuthHarness(t *testing.T, requireVerification bool) *authHarness {
	t.Helper()

	userRepo := newFakeRepo()
	refreshRepo := newFakeRefreshRepo()
	verificationRepo := newFakeVerificationRepo()
	resetRepo := newFakePasswordResetRepo()
	mail := &recordingMailer{}
	manager := jwt.NewManager(strings.Repeat("x", 32), "gobook-test", 15*time.Minute)

	users := services.NewUserService(userRepo, refreshRepo)

	harness := &authHarness{
		auth: services.NewAuthService(
			users, userRepo, refreshRepo, verificationRepo, resetRepo,
			manager, mail,
			services.AuthOptions{
				AppName:                  "GoBook",
				AppURL:                   "http://localhost:8080",
				RequireEmailVerification: requireVerification,
				VerificationTTL:          24 * time.Hour,
				RefreshTTL:               720 * time.Hour,
				PasswordResetTTL:         60 * time.Minute,
				PasswordResetURL:         "http://localhost:3000/reset-password",
				// Run background mail inline: production dispatches it to a
				// goroutine (so /password/forgot leaks no timing signal), which
				// would otherwise race these assertions.
				Dispatcher: func(f func()) { f() },
			},
		),
		users:          userRepo,
		refreshTokens:  refreshRepo,
		verifications:  verificationRepo,
		passwordResets: resetRepo,
		mail:           mail,
		jwt:            manager,
		userService:    users,
	}

	// Same wiring the container performs: creation and address changes both
	// route their verification email through AuthService.
	users.OnEmailNeedsVerification(harness.auth.HandleEmailNeedsVerification)

	return harness
}

func (h *authHarness) register(t *testing.T, email string) *models.User {
	t.Helper()

	user, err := h.auth.Register(context.Background(), requests.RegisterRequest{
		Name: "Toufiq", Email: email, Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return user
}

// -------------------------------------------------------------------- tests

func TestRegisterCreatesUnverifiedUserAndSendsMail(t *testing.T) {
	h := newAuthHarness(t, true)
	user := h.register(t, "new@example.com")

	if user.IsVerified() {
		t.Error("a freshly registered user must not be verified")
	}
	if user.Role != models.RoleUser {
		t.Errorf("expected role %q, got %q", models.RoleUser, user.Role)
	}
	if len(h.mail.sent) != 1 {
		t.Fatalf("expected 1 verification email, got %d", len(h.mail.sent))
	}
	if h.mail.sent[0].To != "new@example.com" {
		t.Errorf("mail sent to %q", h.mail.sent[0].To)
	}
	if !strings.Contains(h.mail.sent[0].Body, "/api/v1/auth/verify-email?token=") {
		t.Error("verification email does not contain a verify link")
	}
}

func TestRegisterSendsNoMailWhenVerificationDisabled(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "quiet@example.com")

	if len(h.mail.sent) != 0 {
		t.Errorf("expected no mail with verification off, got %d", len(h.mail.sent))
	}
}

// A mail outage must not fail the registration — the account is already usable.
func TestRegisterSucceedsWhenMailerFails(t *testing.T) {
	h := newAuthHarness(t, true)
	h.mail.err = context.DeadlineExceeded

	if _, err := h.auth.Register(context.Background(), requests.RegisterRequest{
		Name: "Toufiq", Email: "down@example.com", Password: "supersecret",
	}); err != nil {
		t.Fatalf("registration should survive a mailer failure, got: %v", err)
	}
}

func TestLoginIssuesUsableTokenPair(t *testing.T) {
	h := newAuthHarness(t, false)
	user := h.register(t, "login@example.com")

	pair, err := h.auth.Login(context.Background(), "login@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("login returned an empty token")
	}
	if pair.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", pair.TokenType)
	}

	claims, err := h.jwt.Parse(pair.AccessToken)
	if err != nil {
		t.Fatalf("issued access token does not validate: %v", err)
	}
	gotID, err := claims.UserID()
	if err != nil {
		t.Fatalf("subject: %v", err)
	}
	if gotID != user.ID {
		t.Errorf("token subject = %d, want %d", gotID, user.ID)
	}
}

// The refresh token is handed out in plaintext but must only ever be stored
// hashed — a database leak must not yield presentable credentials.
func TestRefreshTokenIsStoredHashedOnly(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "hash@example.com")

	pair, err := h.auth.Login(context.Background(), "hash@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	for _, stored := range h.refreshTokens.tokens {
		if stored.TokenHash == pair.RefreshToken {
			t.Fatal("refresh token was stored in plaintext")
		}
		if len(stored.TokenHash) != 64 {
			t.Errorf("expected a 64-char sha256 hex hash, got %d chars", len(stored.TokenHash))
		}
	}
}

func TestRefreshRotatesAndRevokesTheOldToken(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "rotate@example.com")

	first, err := h.auth.Login(context.Background(), "rotate@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	second, err := h.auth.Refresh(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh must issue a new token, not reuse the old one")
	}

	// Replaying the consumed token must fail.
	if _, err := h.auth.Refresh(context.Background(), first.RefreshToken); err == nil {
		t.Fatal("a rotated refresh token must not be reusable")
	} else {
		assertStatus(t, err, 401)
	}

	// The newly issued one still works.
	if _, err := h.auth.Refresh(context.Background(), second.RefreshToken); err != nil {
		t.Errorf("the current refresh token should still work: %v", err)
	}
}

func TestRefreshRejectsUnknownAndExpiredTokens(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "expire@example.com")

	pair, err := h.auth.Login(context.Background(), "expire@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	_, unknown := h.auth.Refresh(context.Background(), "not-a-real-token")
	assertStatus(t, unknown, 401)

	_, empty := h.auth.Refresh(context.Background(), "")
	assertStatus(t, empty, 401)

	// Age the stored token past its expiry.
	for _, stored := range h.refreshTokens.tokens {
		stored.ExpiresAt = time.Now().Add(-time.Minute)
	}
	_, expired := h.auth.Refresh(context.Background(), pair.RefreshToken)
	assertStatus(t, expired, 401)

	// Every rejection reason must look identical from the outside.
	if unknown.Error() != expired.Error() {
		t.Errorf("refresh errors are distinguishable: %q vs %q", unknown, expired)
	}
}

func TestLogoutRevokesTokens(t *testing.T) {
	h := newAuthHarness(t, false)
	user := h.register(t, "logout@example.com")

	pair, err := h.auth.Login(context.Background(), "logout@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := h.auth.Logout(context.Background(), user.ID, pair.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := h.auth.Refresh(context.Background(), pair.RefreshToken); err == nil {
		t.Fatal("a logged-out refresh token must not be reusable")
	}

	// Logout is idempotent and silent about unknown tokens.
	if err := h.auth.Logout(context.Background(), user.ID, "never-existed"); err != nil {
		t.Errorf("logout of an unknown token should be a no-op, got: %v", err)
	}

	// LogoutAll kills every outstanding session.
	a, _ := h.auth.Login(context.Background(), "logout@example.com", "supersecret")
	b, _ := h.auth.Login(context.Background(), "logout@example.com", "supersecret")
	if err := h.auth.LogoutAll(context.Background(), user.ID); err != nil {
		t.Fatalf("logout all: %v", err)
	}
	for i, p := range []*services.TokenPair{a, b} {
		if _, err := h.auth.Refresh(context.Background(), p.RefreshToken); err == nil {
			t.Errorf("session %d survived LogoutAll", i)
		}
	}
}

func TestVerifyEmailStampsTheUser(t *testing.T) {
	h := newAuthHarness(t, true)
	user := h.register(t, "verify@example.com")

	plaintext := extractToken(t, h.mail.sent[0].Body)

	verified, err := h.auth.VerifyEmail(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !verified.IsVerified() {
		t.Fatal("user should be verified after redeeming the token")
	}

	stored, err := h.users.FindByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !stored.IsVerified() {
		t.Error("verification was not persisted")
	}

	// Clicking the same link twice is normal human behaviour, not an error.
	if _, err := h.auth.VerifyEmail(context.Background(), plaintext); err != nil {
		t.Errorf("re-verifying should succeed idempotently, got: %v", err)
	}
}

func TestVerifyEmailRejectsBadAndExpiredTokens(t *testing.T) {
	h := newAuthHarness(t, true)
	h.register(t, "badtoken@example.com")

	_, unknown := h.auth.VerifyEmail(context.Background(), "nonsense")
	assertStatus(t, unknown, 400)

	_, empty := h.auth.VerifyEmail(context.Background(), "")
	assertStatus(t, empty, 400)

	plaintext := extractToken(t, h.mail.sent[0].Body)
	for _, stored := range h.verifications.tokens {
		stored.ExpiresAt = time.Now().Add(-time.Minute)
	}
	_, expired := h.auth.VerifyEmail(context.Background(), plaintext)
	assertStatus(t, expired, 400)
}

// Verifying must refresh the claim: a token minted before verification says
// false, one minted after says true.
func TestAccessTokenCarriesVerifiedClaim(t *testing.T) {
	h := newAuthHarness(t, true)
	h.register(t, "claim@example.com")

	before, err := h.auth.Login(context.Background(), "claim@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	claims, err := h.jwt.Parse(before.AccessToken)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Verified {
		t.Error("token issued before verification should carry verified=false")
	}

	if _, err := h.auth.VerifyEmail(context.Background(), extractToken(t, h.mail.sent[0].Body)); err != nil {
		t.Fatalf("verify: %v", err)
	}

	after, err := h.auth.Refresh(context.Background(), before.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	claims, err = h.jwt.Parse(after.AccessToken)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !claims.Verified {
		t.Error("token issued after verification should carry verified=true")
	}
}

func TestResendVerificationInvalidatesThePreviousLink(t *testing.T) {
	h := newAuthHarness(t, true)
	user := h.register(t, "resend@example.com")

	firstToken := extractToken(t, h.mail.sent[0].Body)

	if err := h.auth.ResendVerification(context.Background(), user.ID); err != nil {
		t.Fatalf("resend: %v", err)
	}
	if len(h.mail.sent) != 2 {
		t.Fatalf("expected a second email, got %d", len(h.mail.sent))
	}

	// The superseded link must no longer work.
	if _, err := h.auth.VerifyEmail(context.Background(), firstToken); err == nil {
		t.Error("the previous verification link should have been invalidated")
	}

	if _, err := h.auth.VerifyEmail(context.Background(), extractToken(t, h.mail.sent[1].Body)); err != nil {
		t.Errorf("the newest link should work: %v", err)
	}
}

func TestResendVerificationRejectsAlreadyVerified(t *testing.T) {
	h := newAuthHarness(t, true)
	user := h.register(t, "already@example.com")

	if _, err := h.auth.VerifyEmail(context.Background(), extractToken(t, h.mail.sent[0].Body)); err != nil {
		t.Fatalf("verify: %v", err)
	}

	err := h.auth.ResendVerification(context.Background(), user.ID)
	assertStatus(t, err, 409)
}

// ------------------------------------------------------- password reset

func TestForgotPasswordSendsALink(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "forgot@example.com")

	if err := h.auth.ForgotPassword(context.Background(), "forgot@example.com"); err != nil {
		t.Fatalf("forgot: %v", err)
	}

	if len(h.mail.sent) != 1 {
		t.Fatalf("expected 1 reset email, got %d", len(h.mail.sent))
	}
	if !strings.Contains(h.mail.sent[0].Body, "reset-password?token=") {
		t.Error("reset email does not link to the frontend reset form")
	}
	if !strings.Contains(h.mail.sent[0].Subject, "Reset") {
		t.Errorf("unexpected subject: %q", h.mail.sent[0].Subject)
	}
}

// The endpoint must behave identically for addresses that don't exist, or it
// becomes a way to discover which emails have accounts.
func TestForgotPasswordDoesNotRevealWhetherAnAccountExists(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "real@example.com")

	realErr := h.auth.ForgotPassword(context.Background(), "real@example.com")
	fakeErr := h.auth.ForgotPassword(context.Background(), "ghost@example.com")

	if realErr != nil || fakeErr != nil {
		t.Fatalf("both calls must succeed silently: real=%v fake=%v", realErr, fakeErr)
	}
	if len(h.mail.sent) != 1 {
		t.Errorf("only the real address should receive mail, got %d sends", len(h.mail.sent))
	}
}

// A mail outage must not become an observable difference either.
func TestForgotPasswordStaysSilentWhenMailerFails(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "mailfail@example.com")
	h.mail.err = context.DeadlineExceeded

	if err := h.auth.ForgotPassword(context.Background(), "mailfail@example.com"); err != nil {
		t.Fatalf("a mailer failure must not surface to the caller, got: %v", err)
	}
}

func TestResetPasswordChangesTheCredential(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "reset@example.com")

	if err := h.auth.ForgotPassword(context.Background(), "reset@example.com"); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	resetToken := extractResetToken(t, h.mail.sent[0].Body)

	if err := h.auth.ResetPassword(context.Background(), resetToken, "brand-new-password"); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// The old password must stop working...
	if _, err := h.auth.Login(context.Background(), "reset@example.com", "supersecret"); err == nil {
		t.Error("the old password still works after a reset")
	}
	// ...and the new one must start.
	if _, err := h.auth.Login(context.Background(), "reset@example.com", "brand-new-password"); err != nil {
		t.Errorf("the new password does not work: %v", err)
	}
}

// Resetting is often a response to compromise, so every existing session has to
// die — otherwise an attacker's refresh token outlives the reset.
func TestResetPasswordRevokesEveryExistingSession(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "sessions@example.com")

	first, err := h.auth.Login(context.Background(), "sessions@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	second, err := h.auth.Login(context.Background(), "sessions@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := h.auth.ForgotPassword(context.Background(), "sessions@example.com"); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	if err := h.auth.ResetPassword(context.Background(),
		extractResetToken(t, h.mail.sent[0].Body), "brand-new-password"); err != nil {
		t.Fatalf("reset: %v", err)
	}

	for i, pair := range []*services.TokenPair{first, second} {
		if _, err := h.auth.Refresh(context.Background(), pair.RefreshToken); err == nil {
			t.Errorf("session %d survived the password reset", i)
		}
	}
}

func TestResetPasswordTokenIsSingleUse(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "once@example.com")

	if err := h.auth.ForgotPassword(context.Background(), "once@example.com"); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	resetToken := extractResetToken(t, h.mail.sent[0].Body)

	if err := h.auth.ResetPassword(context.Background(), resetToken, "first-new-password"); err != nil {
		t.Fatalf("first reset: %v", err)
	}

	err := h.auth.ResetPassword(context.Background(), resetToken, "second-new-password")
	assertStatus(t, err, 400)

	// The replay must not have changed anything.
	if _, err := h.auth.Login(context.Background(), "once@example.com", "first-new-password"); err != nil {
		t.Errorf("the first reset should still be in effect: %v", err)
	}
}

func TestResetPasswordRejectsBadAndExpiredTokens(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "badreset@example.com")

	if err := h.auth.ForgotPassword(context.Background(), "badreset@example.com"); err != nil {
		t.Fatalf("forgot: %v", err)
	}

	assertStatus(t, h.auth.ResetPassword(context.Background(), "nonsense", "brand-new-password"), 400)
	assertStatus(t, h.auth.ResetPassword(context.Background(), "", "brand-new-password"), 400)

	resetToken := extractResetToken(t, h.mail.sent[0].Body)
	for _, stored := range h.passwordResets.tokens {
		stored.ExpiresAt = time.Now().Add(-time.Minute)
	}
	assertStatus(t, h.auth.ResetPassword(context.Background(), resetToken, "brand-new-password"), 400)
}

// Requesting a second link must retire the first — otherwise every reset ever
// requested stays live until it expires.
func TestRequestingASecondResetInvalidatesTheFirst(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "twice@example.com")

	if err := h.auth.ForgotPassword(context.Background(), "twice@example.com"); err != nil {
		t.Fatalf("first forgot: %v", err)
	}
	firstToken := extractResetToken(t, h.mail.sent[0].Body)

	if err := h.auth.ForgotPassword(context.Background(), "twice@example.com"); err != nil {
		t.Fatalf("second forgot: %v", err)
	}
	secondToken := extractResetToken(t, h.mail.sent[1].Body)

	assertStatus(t, h.auth.ResetPassword(context.Background(), firstToken, "brand-new-password"), 400)

	if err := h.auth.ResetPassword(context.Background(), secondToken, "brand-new-password"); err != nil {
		t.Errorf("the newest link should work: %v", err)
	}
}

// The reset token is handed out in plaintext but must only ever be stored
// hashed, exactly like refresh and verification tokens.
func TestResetTokenIsStoredHashedOnly(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "hashed@example.com")

	if err := h.auth.ForgotPassword(context.Background(), "hashed@example.com"); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	plaintext := extractResetToken(t, h.mail.sent[0].Body)

	for _, stored := range h.passwordResets.tokens {
		if stored.TokenHash == plaintext {
			t.Fatal("reset token was stored in plaintext")
		}
		if len(stored.TokenHash) != 64 {
			t.Errorf("expected a 64-char sha256 hex hash, got %d chars", len(stored.TokenHash))
		}
	}
}

// extractResetToken pulls the token out of a reset email.
func extractResetToken(t *testing.T, body string) string {
	t.Helper()

	_, after, found := strings.Cut(body, "reset-password?token=")
	if !found {
		t.Fatal("no reset link in the email body")
	}
	tok, _, _ := strings.Cut(after, `"`)
	return tok
}

// extractToken pulls the ?token=... value out of the generated email body.
func extractToken(t *testing.T, body string) string {
	t.Helper()

	_, after, found := strings.Cut(body, "verify-email?token=")
	if !found {
		t.Fatal("no verification link in the email body")
	}
	token, _, _ := strings.Cut(after, `"`)
	return token
}

// ------------------------------------------------- audit regressions

// Rotation is only meaningful if a token can be spent exactly once. The check
// in Refresh is not the gate — the revoking UPDATE is — so a fake that reports
// "I did not do the revoking" must cause a rejection.
func TestRefreshRejectsTheLoserOfARotationRace(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "race@example.com")

	pair, err := h.auth.Login(context.Background(), "race@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Simulate the competing request winning: the row is already revoked by the
	// time our caller reaches the UPDATE.
	for _, stored := range h.refreshTokens.tokens {
		revoked := time.Now()
		stored.RevokedAt = &revoked
	}

	_, err = h.auth.Refresh(context.Background(), pair.RefreshToken)
	assertStatus(t, err, 401)
}

// Same property for reset links: whoever loses the MarkUsed race must not go on
// to change the password.
func TestResetPasswordRejectsTheLoserOfARace(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "resetrace@example.com")

	if err := h.auth.ForgotPassword(context.Background(), "resetrace@example.com"); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	resetToken := extractResetToken(t, h.mail.sent[0].Body)

	for _, stored := range h.passwordResets.tokens {
		used := time.Now()
		stored.UsedAt = &used
	}

	err := h.auth.ResetPassword(context.Background(), resetToken, "brand-new-password")
	assertStatus(t, err, 400)

	// The original password must be untouched.
	if _, err := h.auth.Login(context.Background(), "resetrace@example.com", "supersecret"); err != nil {
		t.Errorf("the losing reset changed the password anyway: %v", err)
	}
}

// A soft-deleted user still occupies the unique index on email, so the taken
// check has to see deleted rows or the INSERT fails with a raw 500.
func TestRegisterReportsConflictForASoftDeletedEmail(t *testing.T) {
	h := newAuthHarness(t, false)
	user := h.register(t, "gone@example.com")

	if err := h.users.Delete(context.Background(), user.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := h.auth.Register(context.Background(), requests.RegisterRequest{
		Name: "Someone Else", Email: "gone@example.com", Password: "supersecret",
	})
	assertStatus(t, err, 409)
}

// ------------------------------------------- third-audit regressions

// A new address has not been proved, so the verified stamp must not carry over
// — otherwise verifying a throwaway then switching addresses bypasses the gate.
func TestChangingEmailClearsTheVerifiedStamp(t *testing.T) {
	h := newAuthHarness(t, true)
	user := h.register(t, "before@example.com")

	if _, err := h.auth.VerifyEmail(context.Background(), extractToken(t, h.mail.sent[0].Body)); err != nil {
		t.Fatalf("verify: %v", err)
	}

	users := h.userService
	newEmail := "after@example.com"
	updated, err := users.Update(context.Background(), user.ID, requests.UpdateUserRequest{Email: &newEmail})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.IsVerified() {
		t.Error("the verified stamp survived a change of address")
	}
}

// Changing a password must close the sessions the old one opened — the same
// property ResetPassword has, on the direct path an admin would use.
func TestChangingPasswordRevokesSessions(t *testing.T) {
	h := newAuthHarness(t, false)
	user := h.register(t, "pwchange@example.com")

	pair, err := h.auth.Login(context.Background(), "pwchange@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	users := h.userService
	newPassword := "a-different-password"
	if _, err := users.Update(context.Background(), user.ID, requests.UpdateUserRequest{Password: &newPassword}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := h.auth.Refresh(context.Background(), pair.RefreshToken); err == nil {
		t.Error("a session survived the password change")
	}
}

// Disabling an account should not leave it able to mint fresh access tokens.
func TestDeactivatingRevokesSessions(t *testing.T) {
	h := newAuthHarness(t, false)
	user := h.register(t, "deactivate@example.com")

	pair, err := h.auth.Login(context.Background(), "deactivate@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	users := h.userService
	inactive := false
	if _, err := users.Update(context.Background(), user.ID, requests.UpdateUserRequest{IsActive: &inactive}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := h.auth.Refresh(context.Background(), pair.RefreshToken); err == nil {
		t.Error("a disabled account could still refresh")
	}
}

// Presenting somebody else's refresh token must not end their session.
func TestLogoutCannotRevokeAnotherUsersToken(t *testing.T) {
	h := newAuthHarness(t, false)
	h.register(t, "victim@example.com")
	attacker := h.register(t, "attacker@example.com")

	victimPair, err := h.auth.Login(context.Background(), "victim@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// The attacker presents the victim's token as their own logout request.
	if err := h.auth.Logout(context.Background(), attacker.ID, victimPair.RefreshToken); err != nil {
		t.Fatalf("logout should stay silent, got: %v", err)
	}

	if _, err := h.auth.Refresh(context.Background(), victimPair.RefreshToken); err != nil {
		t.Errorf("the victim's session was revoked by another user: %v", err)
	}
}

// ------------------------------------------ fourth-audit regressions

// Every creation path must ask for proof of the address, not just
// self-service registration — an admin-created account was previously
// unverified with no link and no way to know why.
func TestAdminCreatedUsersAlsoGetAVerificationEmail(t *testing.T) {
	h := newAuthHarness(t, true)

	if _, err := h.userService.Create(context.Background(), requests.CreateUserRequest{
		Name: "Made By Admin", Email: "byadmin@example.com", Password: "supersecret", Role: models.RoleAdmin,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(h.mail.sent) != 1 {
		t.Fatalf("expected a verification email for an admin-created user, got %d", len(h.mail.sent))
	}
	if h.mail.sent[0].To != "byadmin@example.com" {
		t.Errorf("sent to %q", h.mail.sent[0].To)
	}
}

// Sessions opened while the old address counted as verified must not outlive
// the change — their access tokens still carry verified: true.
func TestChangingEmailRevokesSessions(t *testing.T) {
	h := newAuthHarness(t, false)
	user := h.register(t, "rotate-email@example.com")

	pair, err := h.auth.Login(context.Background(), "rotate-email@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	newEmail := "rotated@example.com"
	if _, err := h.userService.Update(context.Background(), user.ID,
		requests.UpdateUserRequest{Email: &newEmail}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := h.auth.Refresh(context.Background(), pair.RefreshToken); err == nil {
		t.Error("a session survived the address change")
	}
}

func TestCurrentPasswordMustMatchWhenSupplied(t *testing.T) {
	h := newAuthHarness(t, false)
	user := h.register(t, "confirm@example.com")

	wrong, newPassword := "not-my-password", "a-brand-new-one"
	_, err := h.userService.Update(context.Background(), user.ID, requests.UpdateUserRequest{
		Password: &newPassword, CurrentPassword: &wrong,
	})
	assertStatus(t, err, 403)

	// The password must be unchanged after a rejected attempt.
	if _, err := h.auth.Login(context.Background(), "confirm@example.com", "supersecret"); err != nil {
		t.Errorf("the original password stopped working after a rejected change: %v", err)
	}

	right := "supersecret"
	if _, err := h.userService.Update(context.Background(), user.ID, requests.UpdateUserRequest{
		Password: &newPassword, CurrentPassword: &right,
	}); err != nil {
		t.Fatalf("update with the correct current password: %v", err)
	}
	if _, err := h.auth.Login(context.Background(), "confirm@example.com", newPassword); err != nil {
		t.Errorf("the new password does not work: %v", err)
	}
}

// ------------------------------------------- fifth-audit regressions

// The current password is required regardless of role — an admin changing
// their own is the most privileged account in the system, not the least
// protected one.
func TestChangePasswordRequiresTheCurrentOneForEveryRole(t *testing.T) {
	for _, role := range []string{models.RoleUser, models.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			h := newAuthHarness(t, false)

			user, err := h.userService.Create(context.Background(), requests.CreateUserRequest{
				Name: "Somebody", Email: role + "@example.com", Password: "supersecret", Role: role,
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}

			_, err = h.auth.ChangePassword(context.Background(), user.ID, "wrong-one", "a-new-password")
			assertStatus(t, err, 403)

			if _, err := h.auth.ChangePassword(context.Background(), user.ID, "supersecret", "a-new-password"); err != nil {
				t.Fatalf("change with the correct current password: %v", err)
			}
			if _, err := h.auth.Login(context.Background(), role+"@example.com", "a-new-password"); err != nil {
				t.Errorf("the new password does not work: %v", err)
			}
		})
	}
}

// Changing your password must not sign you out of the device you are using,
// while every other session dies.
func TestChangePasswordReturnsWorkingTokensAndKillsTheRest(t *testing.T) {
	h := newAuthHarness(t, false)
	user := h.register(t, "keepme@example.com")

	other, err := h.auth.Login(context.Background(), "keepme@example.com", "supersecret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	fresh, err := h.auth.ChangePassword(context.Background(), user.ID, "supersecret", "a-new-password")
	if err != nil {
		t.Fatalf("change: %v", err)
	}

	if fresh.AccessToken == "" || fresh.RefreshToken == "" {
		t.Fatal("change should hand back a usable pair")
	}
	if _, err := h.auth.Refresh(context.Background(), fresh.RefreshToken); err != nil {
		t.Errorf("the reissued refresh token does not work: %v", err)
	}
	if _, err := h.auth.Refresh(context.Background(), other.RefreshToken); err == nil {
		t.Error("a pre-existing session survived the password change")
	}
}
