// Package services holds business rules. It knows nothing about HTTP: no Gin,
// no status codes, no request structs beyond the DTOs. That's what makes the
// same logic reusable from a CLI command, a queue worker or a test.
package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
	"github.com/scriptertoufiq/go-mvc/internal/repositories"
	"github.com/scriptertoufiq/go-mvc/internal/requests"
	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/hash"
	"github.com/scriptertoufiq/go-mvc/pkg/pagination"
)

type UserService struct {
	repo repositories.UserRepository

	// refreshTokens is here so that changing a credential can invalidate the
	// sessions that credential opened. Without it, "change the password" and
	// "lock the account out" would be two different operations, and only the
	// password-reset flow would do both.
	refreshTokens repositories.RefreshTokenRepository

	// needsVerification fires whenever an address has not been proved — a new
	// account, or a changed address. It is a callback rather than a direct
	// dependency because AuthService already depends on UserService, and
	// wiring it the other way would be an import cycle.
	needsVerification func(context.Context, *models.User)
}

func NewUserService(
	repo repositories.UserRepository,
	refreshTokens repositories.RefreshTokenRepository,
) *UserService {
	return &UserService{repo: repo, refreshTokens: refreshTokens}
}

// OnEmailNeedsVerification registers the hook fired when an address needs
// proving. AuthService supplies it; it is a no-op when verification is off.
func (s *UserService) OnEmailNeedsVerification(fn func(context.Context, *models.User)) {
	s.needsVerification = fn
}

func (s *UserService) List(ctx context.Context, p pagination.Params) ([]models.User, pagination.Meta, error) {
	users, total, err := s.repo.Paginate(ctx, p)
	if err != nil {
		return nil, pagination.Meta{}, apperror.Internal(err)
	}
	return users, pagination.NewMeta(p, total), nil
}

func (s *UserService) Get(ctx context.Context, id uint) (*models.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("User")
		}
		return nil, apperror.Internal(err)
	}
	return user, nil
}

func (s *UserService) Create(ctx context.Context, req requests.CreateUserRequest) (*models.User, error) {
	email := normaliseEmail(req.Email)

	taken, err := s.repo.EmailTaken(ctx, email, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if taken {
		return nil, apperror.Conflict("That email address is already registered.")
	}

	hashed, err := hash.Make(req.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	user := &models.User{
		Name:     strings.TrimSpace(req.Name),
		Email:    email,
		Password: hashed,
		Role:     role,
		IsActive: true,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, apperror.Internal(err)
	}

	// Every creation path lands here — self-service registration and an admin
	// using POST /users alike — so this is the one place that has to ask for
	// proof of the address. Doing it in AuthService.Register instead left
	// admin-created accounts unverified with no way to know why.
	s.requestVerification(ctx, user)

	return user, nil
}

func (s *UserService) Update(ctx context.Context, id uint, req requests.UpdateUserRequest) (*models.User, error) {
	user, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	var emailChanged bool

	if req.Email != nil {
		email := normaliseEmail(*req.Email)
		taken, err := s.repo.EmailTaken(ctx, email, user.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if taken {
			return nil, apperror.Conflict("That email address is already registered.")
		}

		emailChanged = email != user.Email
		user.Email = email
	}

	if req.Name != nil {
		user.Name = strings.TrimSpace(*req.Name)
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Password != nil {
		// When the caller supplied their current password, it must match.
		// Controllers require it for self-service changes; an admin acting on
		// someone else's account has no current password to offer.
		if req.CurrentPassword != nil && !hash.Check(user.Password, *req.CurrentPassword) {
			return nil, apperror.Forbidden("Your current password is incorrect.")
		}

		hashed, err := hash.Make(*req.Password)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		user.Password = hashed
	}

	// A new address has not been proved, so the old proof cannot carry over.
	// Without this, anyone could verify a throwaway address and then switch to
	// one they do not control while still counting as verified.
	if emailChanged {
		user.EmailVerifiedAt = nil
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, apperror.Internal(err)
	}

	// Changing the password, or disabling the account, must also close the
	// sessions opened under the old state. This is the single choke point for
	// password changes — AuthService.ResetPassword routes through here — so
	// doing it once here keeps the two paths from drifting apart.
	// An address change joins the list: the sessions were opened while the
	// account still counted as verified, and their access tokens still carry
	// that claim.
	deactivated := req.IsActive != nil && !*req.IsActive
	if req.Password != nil || deactivated || emailChanged {
		if err := s.revokeSessions(ctx, user.ID); err != nil {
			return nil, err
		}
	}

	if emailChanged {
		s.requestVerification(ctx, user)
	}

	return user, nil
}

// requestVerification asks the auth layer to send a proof-of-address link.
// Fire and forget: a mail problem must not fail the write that triggered it.
func (s *UserService) requestVerification(ctx context.Context, user *models.User) {
	if s.needsVerification != nil && !user.IsVerified() {
		s.needsVerification(ctx, user)
	}
}

func (s *UserService) Delete(ctx context.Context, id uint) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperror.Internal(err)
	}

	// A deleted account must not keep refreshing its way to new access tokens.
	// Refresh already rejects deleted users, so this is belt-and-braces — but
	// it also means the rows stop being live credentials immediately.
	return s.revokeSessions(ctx, id)
}

// revokeSessions invalidates every refresh token a user holds.
func (s *UserService) revokeSessions(ctx context.Context, userID uint) error {
	if s.refreshTokens == nil {
		return nil
	}
	if err := s.refreshTokens.RevokeAllForUser(ctx, userID, time.Now()); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// Authenticate verifies credentials. It returns the same error for "no such
// user" and "wrong password" so the endpoint can't be used to enumerate accounts.
func (s *UserService) Authenticate(ctx context.Context, email, password string) (*models.User, error) {
	user, err := s.repo.FindByEmail(ctx, normaliseEmail(email))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.Unauthorized("Invalid credentials.")
		}
		return nil, apperror.Internal(err)
	}

	if !hash.Check(user.Password, password) {
		return nil, apperror.Unauthorized("Invalid credentials.")
	}
	if !user.IsActive {
		return nil, apperror.Forbidden("This account is disabled.")
	}

	return user, nil
}

func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
