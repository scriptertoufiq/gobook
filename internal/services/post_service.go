package services

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/go-mvc/internal/models"
	"github.com/scriptertoufiq/go-mvc/internal/repositories"
	"github.com/scriptertoufiq/go-mvc/internal/requests"
	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/cache"
	"github.com/scriptertoufiq/go-mvc/pkg/pagination"
)

// ownedBy reports whether the post belongs to this caller.
//
// Authorship is the only thing that grants write access — an administrator has
// no override here. Role is therefore not a parameter: a caller id is the whole
// of the input, and there is no branch for anyone else to fall down.
//
// The check cannot live in route middleware the way it does for /users/:id.
// There, `:id` *is* the user id, so RequireSelfOrRole can compare the two
// before the handler runs. Here `:id` is the post id, and who owns it is only
// knowable once the row is loaded — so it belongs in this service.
func ownedBy(post *models.Post, callerID uint) bool {
	return post.UserID == callerID
}

type PostService struct {
	repo  repositories.PostRepository
	cache cache.Cache
	ttl   time.Duration
}

func NewPostService(repo repositories.PostRepository, store cache.Cache, ttl time.Duration) *PostService {
	return &PostService{repo: repo, cache: store, ttl: ttl}
}

// postCacheKey is the single place the key format lives, so the read that
// populates it and the writes that clear it can never drift apart.
func postCacheKey(id uint) string {
	return cache.Key("posts", "show", id)
}

// forget drops a post's cached copy.
//
// Called after a write succeeds. A failure here is logged rather than
// returned: the write is already durable, so reporting an error would invite
// the client to retry an operation that worked. The cost is a stale read
// bounded by the TTL, which is why the default is short.
func (s *PostService) forget(ctx context.Context, id uint) {
	if err := s.cache.Delete(ctx, postCacheKey(id)); err != nil {
		log.Printf("cache: could not invalidate post %d, reads may be stale until it expires: %v", id, err)
	}
}

// List returns a page of posts, optionally narrowed to a single author.
func (s *PostService) List(
	ctx context.Context,
	p pagination.Params,
	authorID uint,
) ([]models.Post, pagination.Meta, error) {
	posts, total, err := s.repo.Paginate(ctx, p, authorID)
	if err != nil {
		return nil, pagination.Meta{}, apperror.Internal(err)
	}
	return posts, pagination.NewMeta(p, total), nil
}

// Get returns a single post, served from cache when it is there.
//
// Read-through: a miss falls to the database and stores the result, so the
// next reader is spared the query. A value type rather than a pointer is
// cached, which makes a decoded `null` impossible — there is no nil for a
// caller to dereference.
//
// Only successful lookups are cached. RememberFrom returns early when compute
// fails, so a 404 is never stored — otherwise a post created moments after
// somebody probed for it would stay "missing" for the whole TTL.
// It also reports which path answered, so the handler can tell the caller
// whether they were served from Redis or from MySQL.
func (s *PostService) Get(ctx context.Context, id uint) (*models.Post, cache.Source, error) {
	post, source, err := cache.RememberFrom(ctx, s.cache, postCacheKey(id), s.ttl,
		func() (models.Post, error) {
			found, err := s.fetch(ctx, id)
			if err != nil {
				return models.Post{}, err
			}
			return *found, nil
		})
	if err != nil {
		return nil, source, err
	}

	return &post, source, nil
}

// fetch reads a post straight from the database, bypassing the cache. It is
// what Get falls back to on a miss, and what the write paths use so an edit is
// never applied to a stale copy.
func (s *PostService) fetch(ctx context.Context, id uint) (*models.Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Post")
		}
		return nil, apperror.Internal(err)
	}
	return post, nil
}

// Create publishes a post authored by the caller. The author is a parameter
// rather than a request field precisely so it cannot be spoofed.
func (s *PostService) Create(
	ctx context.Context,
	authorID uint,
	req requests.CreatePostRequest,
) (*models.Post, error) {
	post := &models.Post{
		UserID:  authorID,
		Title:   strings.TrimSpace(req.Title),
		Content: strings.TrimSpace(req.Content),
	}

	if err := s.repo.Create(ctx, post); err != nil {
		return nil, apperror.Internal(err)
	}
	return post, nil
}

// Update edits a post. Only its author may do so.
func (s *PostService) Update(
	ctx context.Context,
	id uint,
	callerID uint,
	req requests.UpdatePostRequest,
) (*models.Post, error) {
	// fetch, not Get: an update must be computed from the row as it is in the
	// database, never from a cached copy that may already be behind.
	post, err := s.fetch(ctx, id)
	if err != nil {
		return nil, err
	}

	if !ownedBy(post, callerID) {
		return nil, errNotYourPost()
	}

	if req.Title != nil {
		post.Title = strings.TrimSpace(*req.Title)
	}
	if req.Content != nil {
		post.Content = strings.TrimSpace(*req.Content)
	}

	if err := s.repo.Update(ctx, post); err != nil {
		return nil, apperror.Internal(err)
	}

	// Invalidate rather than overwrite. Writing the new value back would race
	// with any concurrent update, and could leave the cache holding a version
	// the database never had; dropping the key makes the next read authoritative.
	s.forget(ctx, id)

	return post, nil
}

// Delete soft-deletes a post. Only its author may do so.
func (s *PostService) Delete(ctx context.Context, id uint, callerID uint) error {
	post, err := s.fetch(ctx, id)
	if err != nil {
		return err
	}

	if !ownedBy(post, callerID) {
		return errNotYourPost()
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return apperror.Internal(err)
	}

	// Without this a deleted post keeps being served until its TTL runs out,
	// which is the most visible way a cache can be wrong.
	s.forget(ctx, id)

	return nil
}

// errNotYourPost is a 403 rather than a 404-in-disguise.
//
// Elsewhere this codebase hides existence — an unknown refresh token and a
// revoked one return the same error, because distinguishing them helps an
// attacker. That reasoning does not apply here: every authenticated caller may
// already read every post, so the row's existence is not a secret, and a plain
// "not yours" is the more useful answer. Administrators see this too.
func errNotYourPost() *apperror.Error {
	return apperror.Forbidden("You may only modify your own posts.")
}
