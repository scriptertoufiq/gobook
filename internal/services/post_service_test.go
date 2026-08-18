package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
	"github.com/scriptertoufiq/go-mvc/internal/repositories"
	"github.com/scriptertoufiq/go-mvc/internal/requests"
	"github.com/scriptertoufiq/go-mvc/internal/services"
	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/cache"
	"github.com/scriptertoufiq/go-mvc/pkg/pagination"
)

// fakePostRepo is an in-memory stand-in that counts reads, which is how these
// tests tell a cache hit from a database round trip.
type fakePostRepo struct {
	posts     map[uint]*models.Post
	nextID    uint
	findCalls int
}

var _ repositories.PostRepository = (*fakePostRepo)(nil)

func newFakePostRepo() *fakePostRepo {
	return &fakePostRepo{posts: map[uint]*models.Post{}, nextID: 1}
}

func (r *fakePostRepo) Create(_ context.Context, post *models.Post) error {
	post.ID = r.nextID
	r.nextID++
	copied := *post
	r.posts[post.ID] = &copied
	return nil
}

func (r *fakePostRepo) Update(_ context.Context, post *models.Post) error {
	copied := *post
	r.posts[post.ID] = &copied
	return nil
}

func (r *fakePostRepo) Delete(_ context.Context, id uint) error {
	delete(r.posts, id)
	return nil
}

func (r *fakePostRepo) FindByID(_ context.Context, id uint) (*models.Post, error) {
	r.findCalls++
	post, ok := r.posts[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copied := *post
	return &copied, nil
}

func (r *fakePostRepo) Paginate(_ context.Context, _ pagination.Params, _ uint) ([]models.Post, int64, error) {
	out := make([]models.Post, 0, len(r.posts))
	for _, p := range r.posts {
		out = append(out, *p)
	}
	return out, int64(len(out)), nil
}

// newCachedPostService wires the service to an in-process Redis.
func newCachedPostService(t *testing.T) (*services.PostService, *fakePostRepo) {
	t.Helper()

	server := miniredis.RunT(t)
	store, err := cache.NewRedis(cache.Options{
		Addr:        server.Addr(),
		Prefix:      "test",
		DialTimeout: 2 * time.Second,
		Timeout:     2 * time.Second,
		PoolSize:    4,
	})
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	repo := newFakePostRepo()
	return services.NewPostService(repo, store, 5*time.Minute), repo
}

func seedPost(t *testing.T, svc *services.PostService, author uint, title string) *models.Post {
	t.Helper()

	post, err := svc.Create(context.Background(), author, requests.CreatePostRequest{
		Title:   title,
		Content: "original content",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return post
}

func TestGetServesTheSecondReadFromCache(t *testing.T) {
	svc, repo := newCachedPostService(t)
	ctx := context.Background()
	post := seedPost(t, svc, 1, "Cached post")

	first, err := svc.Get(ctx, post.ID)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	afterFirst := repo.findCalls

	second, err := svc.Get(ctx, post.ID)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}

	if repo.findCalls != afterFirst {
		t.Errorf("second read hit the database: findCalls went %d -> %d", afterFirst, repo.findCalls)
	}
	if first.ID != second.ID || first.Title != second.Title || first.Content != second.Content {
		t.Errorf("cached copy differs from the stored one:\n  %+v\n  %+v", first, second)
	}
}

func TestCachedPostSurvivesRoundTripIntact(t *testing.T) {
	svc, _ := newCachedPostService(t)
	ctx := context.Background()
	post := seedPost(t, svc, 42, "Round trip")

	_, _ = svc.Get(ctx, post.ID) // populate
	got, err := svc.Get(ctx, post.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	switch {
	case got.ID != post.ID:
		t.Errorf("id: got %d want %d", got.ID, post.ID)
	case got.UserID != 42:
		t.Errorf("user_id: got %d want 42", got.UserID)
	case got.Title != "Round trip":
		t.Errorf("title: got %q", got.Title)
	case got.Content != "original content":
		t.Errorf("content: got %q", got.Content)
	}
}

func TestUpdateInvalidatesTheCachedCopy(t *testing.T) {
	svc, repo := newCachedPostService(t)
	ctx := context.Background()
	post := seedPost(t, svc, 1, "Before")

	_, _ = svc.Get(ctx, post.ID) // populate the cache

	updated := "After the edit"
	if _, err := svc.Update(ctx, post.ID, 1, requests.UpdatePostRequest{Title: &updated}); err != nil {
		t.Fatalf("update: %v", err)
	}

	before := repo.findCalls
	got, err := svc.Get(ctx, post.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}

	if got.Title != updated {
		t.Errorf("served a stale title after an edit: got %q, want %q", got.Title, updated)
	}
	if repo.findCalls == before {
		t.Error("expected the read to fall through to the database after invalidation")
	}
}

func TestDeleteInvalidatesSoThePostStopsBeingServed(t *testing.T) {
	svc, _ := newCachedPostService(t)
	ctx := context.Background()
	post := seedPost(t, svc, 1, "Doomed")

	_, _ = svc.Get(ctx, post.ID) // populate

	if err := svc.Delete(ctx, post.ID, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// The most visible way a cache can be wrong: still answering for a row
	// that no longer exists.
	if _, err := svc.Get(ctx, post.ID); err == nil {
		t.Fatal("a deleted post was still served from cache")
	}
}

func TestMissesAreNotCached(t *testing.T) {
	svc, repo := newCachedPostService(t)
	ctx := context.Background()

	if _, err := svc.Get(ctx, 999); err == nil {
		t.Fatal("expected a not-found error")
	}
	afterFirst := repo.findCalls

	if _, err := svc.Get(ctx, 999); err == nil {
		t.Fatal("expected a not-found error")
	}

	// If a 404 were cached, a post created moments later would stay invisible
	// for the whole TTL.
	if repo.findCalls == afterFirst {
		t.Error("the miss was cached — the second lookup never reached the repository")
	}
}

func TestUpdateIsComputedFromTheDatabaseNotTheCache(t *testing.T) {
	svc, repo := newCachedPostService(t)
	ctx := context.Background()
	post := seedPost(t, svc, 1, "Original")

	_, _ = svc.Get(ctx, post.ID) // cache it

	// Something changes the row behind the service's back.
	stored := repo.posts[post.ID]
	stored.Content = "changed underneath"

	newTitle := "Edited"
	got, err := svc.Update(ctx, post.ID, 1, requests.UpdatePostRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if got.Content != "changed underneath" {
		t.Errorf("update was applied to a stale cached copy: content is %q", got.Content)
	}
}

func TestOwnershipIsEnforcedEvenOnACachedPost(t *testing.T) {
	svc, _ := newCachedPostService(t)
	ctx := context.Background()
	post := seedPost(t, svc, 1, "Owned by user 1")

	_, _ = svc.Get(ctx, post.ID) // cache it

	title := "hijacked"
	_, err := svc.Update(ctx, post.ID, 2, requests.UpdatePostRequest{Title: &title})

	var appErr *apperror.Error
	if !asAppError(err, &appErr) || appErr.Status != 403 {
		t.Fatalf("expected a 403 for a non-author, got %v", err)
	}
}

func TestServiceWorksWithCachingSwitchedOff(t *testing.T) {
	repo := newFakePostRepo()
	svc := services.NewPostService(repo, cache.NewNull(), 5*time.Minute)
	ctx := context.Background()

	post := seedPost(t, svc, 1, "Uncached")

	for i := range 3 {
		got, err := svc.Get(ctx, post.ID)
		if err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
		if got.Title != "Uncached" {
			t.Errorf("got %q", got.Title)
		}
	}

	// Every read must reach the repository — that is what "no cache" means.
	if repo.findCalls != 3 {
		t.Errorf("expected 3 database reads with the null cache, got %d", repo.findCalls)
	}
}

func asAppError(err error, target **apperror.Error) bool {
	if err == nil {
		return false
	}
	appErr, ok := err.(*apperror.Error)
	if ok {
		*target = appErr
	}
	return ok
}
