package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/internal/requests"
	"github.com/scriptertoufiq/gobook/internal/services"
	"github.com/scriptertoufiq/gobook/pkg/apperror"
	"github.com/scriptertoufiq/gobook/pkg/hash"
	"github.com/scriptertoufiq/gobook/pkg/pagination"
)

// fakeUserRepo is an in-memory stand-in for the real repository. Because the
// service depends on the UserRepository interface rather than *gorm.DB, the
// whole business layer is testable without a database.
type fakeUserRepo struct {
	users  map[uint]*models.User
	nextID uint
}

func newFakeRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[uint]*models.User{}, nextID: 1}
}

func (f *fakeUserRepo) Create(_ context.Context, u *models.User) error {
	u.ID = f.nextID
	f.nextID++
	f.users[u.ID] = u
	return nil
}

func (f *fakeUserRepo) Update(_ context.Context, u *models.User) error {
	f.users[u.ID] = u
	return nil
}

// Delete soft-deletes, mirroring gorm.DeletedAt. Modelling this faithfully
// matters: the row survives, so it keeps occupying the unique index on email.
func (f *fakeUserRepo) Delete(_ context.Context, id uint) error {
	if u, ok := f.users[id]; ok {
		u.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	}
	return nil
}

func deleted(u *models.User) bool { return u.DeletedAt.Valid }

func (f *fakeUserRepo) FindByID(_ context.Context, id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok && !deleted(u) {
		return u, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*models.User, error) {
	for _, u := range f.users {
		if u.Email == email && !deleted(u) {
			return u, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// EmailTaken counts soft-deleted rows on purpose — the real repository runs
// Unscoped here, because the database's unique index does not forget them.
func (f *fakeUserRepo) EmailTaken(_ context.Context, email string, excludeID uint) (bool, error) {
	for _, u := range f.users {
		if u.Email == email && u.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeUserRepo) Paginate(_ context.Context, _ pagination.Params) ([]models.User, int64, error) {
	out := make([]models.User, 0, len(f.users))
	for _, u := range f.users {
		if !deleted(u) {
			out = append(out, *u)
		}
	}
	return out, int64(len(out)), nil
}

func TestCreateHashesPasswordAndNormalisesEmail(t *testing.T) {
	repo := newFakeRepo()
	svc := services.NewUserService(repo, newFakeRefreshRepo())

	user, err := svc.Create(context.Background(), requests.CreateUserRequest{
		Name:     "  Toufiq  ",
		Email:    "  Toufiq@Example.COM ",
		Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if user.Email != "toufiq@example.com" {
		t.Errorf("email not normalised: got %q", user.Email)
	}
	if user.Name != "Toufiq" {
		t.Errorf("name not trimmed: got %q", user.Name)
	}
	if user.Password == "supersecret" {
		t.Error("password was stored in plain text")
	}
	if !hash.Check(user.Password, "supersecret") {
		t.Error("stored hash does not verify against the original password")
	}
	if user.Role != "user" {
		t.Errorf("expected default role 'user', got %q", user.Role)
	}
}

func TestCreateRejectsDuplicateEmail(t *testing.T) {
	repo := newFakeRepo()
	svc := services.NewUserService(repo, newFakeRefreshRepo())
	req := requests.CreateUserRequest{Name: "Toufiq", Email: "dup@example.com", Password: "supersecret"}

	if _, err := svc.Create(context.Background(), req); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := svc.Create(context.Background(), req)
	assertStatus(t, err, 409)
}

func TestGetMissingUserReturnsNotFound(t *testing.T) {
	svc := services.NewUserService(newFakeRepo(), newFakeRefreshRepo())

	_, err := svc.Get(context.Background(), 999)
	assertStatus(t, err, 404)
}

func TestAuthenticateRejectsBadPasswordWithoutLeakingExistence(t *testing.T) {
	repo := newFakeRepo()
	svc := services.NewUserService(repo, newFakeRefreshRepo())
	_, err := svc.Create(context.Background(), requests.CreateUserRequest{
		Name: "Toufiq", Email: "auth@example.com", Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	_, wrongPass := svc.Authenticate(context.Background(), "auth@example.com", "nope")
	_, noSuchUser := svc.Authenticate(context.Background(), "ghost@example.com", "nope")

	assertStatus(t, wrongPass, 401)
	assertStatus(t, noSuchUser, 401)

	// Identical messages, so the endpoint can't be used to enumerate accounts.
	if wrongPass.Error() != noSuchUser.Error() {
		t.Errorf("auth errors distinguishable: %q vs %q", wrongPass, noSuchUser)
	}
}

func TestUpdateOnlyTouchesSuppliedFields(t *testing.T) {
	repo := newFakeRepo()
	svc := services.NewUserService(repo, newFakeRefreshRepo())
	created, err := svc.Create(context.Background(), requests.CreateUserRequest{
		Name: "Original", Email: "patch@example.com", Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	newName := "Renamed"
	updated, err := svc.Update(context.Background(), created.ID, requests.UpdateUserRequest{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Name != "Renamed" {
		t.Errorf("name not updated: got %q", updated.Name)
	}
	if updated.Email != "patch@example.com" {
		t.Errorf("email should be untouched, got %q", updated.Email)
	}
	if !hash.Check(updated.Password, "supersecret") {
		t.Error("password should be untouched when not supplied")
	}
}

func assertStatus(t *testing.T, err error, want int) {
	t.Helper()

	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperror.Error, got %#v", err)
	}
	if appErr.Status != want {
		t.Errorf("expected status %d, got %d (%s)", want, appErr.Status, appErr.Message)
	}
}
