package services

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/cachekeys"
	appevents "github.com/scriptertoufiq/gobook/internal/events"
	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/internal/repositories"
	"github.com/scriptertoufiq/gobook/internal/requests"
	"github.com/scriptertoufiq/gobook/pkg/apperror"
	"github.com/scriptertoufiq/gobook/pkg/cache"
	"github.com/scriptertoufiq/gobook/pkg/events"
	"github.com/scriptertoufiq/gobook/pkg/pagination"
)

// CommentService owns the conversation rules. Like every service here it knows
// nothing about HTTP.
type CommentService struct {
	repo   repositories.CommentRepository
	posts  *PostService
	cache  cache.Cache
	events *events.Dispatcher
	ttl    time.Duration
}

func NewCommentService(
	repo repositories.CommentRepository,
	posts *PostService,
	store cache.Cache,
	dispatcher *events.Dispatcher,
	ttl time.Duration,
) *CommentService {
	return &CommentService{repo: repo, posts: posts, cache: store, events: dispatcher, ttl: ttl}
}

// generation is the current version of a post's conversation.
//
// Cached page keys embed it, so a write only has to raise this one counter for
// every page of that post to become unreachable — no scan, and posts do not
// invalidate each other. A cache that cannot answer reports 0, which simply
// means every read is a miss.
func (s *CommentService) generation(ctx context.Context, postID uint) int64 {
	n, err := s.cache.Generation(ctx, cachekeys.CommentGeneration(postID))
	if err != nil {
		log.Printf("comments: could not read cache generation for post %d: %v", postID, err)
		return 0
	}
	return n
}

// announce tells the listeners a post's conversation changed. Failures are
// their problem, not the writer's — the comment is already saved.
func (s *CommentService) announce(ctx context.Context, postID, commentID uint, verb string) {
	s.events.Dispatch(ctx, appevents.CommentChanged{PostID: postID, CommentID: commentID, Verb: verb})
}

// Thread is a page of comments plus the reply count for each one, so a client
// can render "view 3 replies" without asking again per comment.
type Thread struct {
	Comments    []models.Comment
	ReplyCounts map[uint]int64
	Meta        pagination.Meta
}

// ForPost returns a page of a post's top-level comments.
//
// Replies are not included. A post with one long argument under it would
// otherwise make every page of its comments enormous, and most readers never
// open most threads.
func (s *CommentService) ForPost(ctx context.Context, postID uint, p pagination.Params) (Thread, error) {
	if _, _, err := s.posts.Get(ctx, postID); err != nil {
		return Thread{}, err
	}

	key := cachekeys.CommentPage(
		postID, s.generation(ctx, postID), p.Page, p.PerPage, p.SortDir, p.Search)

	// Read-through: a miss falls to the database and stores the result, so the
	// next reader of the same page is spared both queries — the page itself and
	// the reply counts that go with it.
	return cache.Remember(ctx, s.cache, key, s.ttl, func() (Thread, error) {
		comments, total, err := s.repo.PaginateForPost(ctx, postID, p)
		if err != nil {
			return Thread{}, apperror.Internal(err)
		}
		return s.withReplyCounts(ctx, comments, total, p)
	})
}

// Replies returns a page of the replies under one comment.
func (s *CommentService) Replies(ctx context.Context, parentID uint, p pagination.Params) (Thread, error) {
	parent, err := s.Get(ctx, parentID)
	if err != nil {
		return Thread{}, err
	}

	// A reply has no replies of its own, so asking for them is a mistaken
	// request rather than an empty one — say so instead of returning nothing.
	if parent.IsReply() {
		return Thread{}, apperror.BadRequest("Replies cannot themselves be replied to, so they have no thread.")
	}

	// Shares the post's generation, so a new comment anywhere in the
	// conversation clears these pages too — which it must, since a reply
	// changes the reply count shown on the top-level page.
	key := cachekeys.ReplyPage(
		parent.PostID, parentID, s.generation(ctx, parent.PostID),
		p.Page, p.PerPage, p.SortDir, p.Search)

	return cache.Remember(ctx, s.cache, key, s.ttl, func() (Thread, error) {
		replies, total, err := s.repo.PaginateReplies(ctx, parentID, p)
		if err != nil {
			return Thread{}, apperror.Internal(err)
		}

		return Thread{
			Comments:    replies,
			ReplyCounts: map[uint]int64{},
			Meta:        pagination.NewMeta(p, total),
		}, nil
	})
}

// withReplyCounts attaches reply counts in one query rather than one per row.
func (s *CommentService) withReplyCounts(
	ctx context.Context,
	comments []models.Comment,
	total int64,
	p pagination.Params,
) (Thread, error) {
	ids := make([]uint, 0, len(comments))
	for i := range comments {
		ids = append(ids, comments[i].ID)
	}

	counts, err := s.repo.ReplyCounts(ctx, ids)
	if err != nil {
		return Thread{}, apperror.Internal(err)
	}

	return Thread{
		Comments:    comments,
		ReplyCounts: counts,
		Meta:        pagination.NewMeta(p, total),
	}, nil
}

// Get reads one comment, from cache when it is there.
//
// Cached because it is not only a public lookup: every reply page begins by
// resolving its parent, so without this a warm reply read still costs a
// database round trip. Keyed by id and invalidated by id, since the change
// event names the comment exactly.
func (s *CommentService) Get(ctx context.Context, id uint) (*models.Comment, error) {
	comment, err := cache.Remember(ctx, s.cache, cachekeys.Comment(id), s.ttl,
		func() (models.Comment, error) {
			found, err := s.fetch(ctx, id)
			if err != nil {
				return models.Comment{}, err
			}
			return *found, nil
		})
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// fetch reads a comment straight from the database. Writes use it so an edit is
// never computed from a cached copy.
func (s *CommentService) fetch(ctx context.Context, id uint) (*models.Comment, error) {
	comment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Comment")
		}
		return nil, apperror.Internal(err)
	}
	return comment, nil
}

// Create adds a comment to a post.
func (s *CommentService) Create(
	ctx context.Context,
	postID, authorID uint,
	req requests.CreateCommentRequest,
) (*models.Comment, error) {
	if _, _, err := s.posts.Get(ctx, postID); err != nil {
		return nil, err
	}

	comment := &models.Comment{
		PostID: postID,
		UserID: authorID,
		Body:   strings.TrimSpace(req.Body),
	}

	if err := s.repo.Create(ctx, comment); err != nil {
		return nil, apperror.Internal(err)
	}

	s.announce(ctx, postID, comment.ID, appevents.CommentCreatedName)

	return comment, nil
}

// Reply adds a comment under another comment.
//
// The post is taken from the parent rather than the caller, so a reply can
// never end up filed under a different post from the comment it answers.
func (s *CommentService) Reply(
	ctx context.Context,
	parentID, authorID uint,
	req requests.CreateCommentRequest,
) (*models.Comment, error) {
	parent, err := s.Get(ctx, parentID)
	if err != nil {
		return nil, err
	}

	// Threads stop at two levels. Replying to a reply attaches to the same
	// thread instead of being refused — the intent is clear, and a person
	// answering a reply means "in this conversation", not "one level deeper".
	target := parent
	if parent.IsReply() {
		target, err = s.Get(ctx, *parent.ParentID)
		if err != nil {
			return nil, err
		}
	}

	comment := &models.Comment{
		PostID:   target.PostID,
		UserID:   authorID,
		ParentID: &target.ID,
		Body:     strings.TrimSpace(req.Body),
	}

	if err := s.repo.Create(ctx, comment); err != nil {
		return nil, apperror.Internal(err)
	}

	s.announce(ctx, comment.PostID, comment.ID, appevents.CommentCreatedName)

	return comment, nil
}

// Update edits a comment. Only its author may do so — the same rule posts
// follow, administrators included in the refusal.
func (s *CommentService) Update(
	ctx context.Context,
	id, callerID uint,
	req requests.UpdateCommentRequest,
) (*models.Comment, error) {
	comment, err := s.fetch(ctx, id)
	if err != nil {
		return nil, err
	}

	if comment.UserID != callerID {
		return nil, errNotYourComment()
	}

	if req.Body != nil {
		comment.Body = strings.TrimSpace(*req.Body)
	}

	if err := s.repo.Update(ctx, comment); err != nil {
		return nil, apperror.Internal(err)
	}

	s.announce(ctx, comment.PostID, comment.ID, appevents.CommentUpdatedName)

	return comment, nil
}

// Delete removes a comment. Only its author may do so.
//
// A top-level comment takes its replies with it. That is the honest behaviour
// for a two-level thread — a reply to a comment that no longer exists has
// nothing to attach to — and it has to be done explicitly: the foreign key
// cascade only fires on a hard delete, and comments soft-delete.
func (s *CommentService) Delete(ctx context.Context, id, callerID uint) error {
	comment, err := s.fetch(ctx, id)
	if err != nil {
		return err
	}

	if comment.UserID != callerID {
		return errNotYourComment()
	}

	if err := s.repo.DeleteWithReplies(ctx, id); err != nil {
		return apperror.Internal(err)
	}

	s.announce(ctx, comment.PostID, id, appevents.CommentDeletedName)

	return nil
}

// CountsForPosts totals the conversation under each post, for a feed.
func (s *CommentService) CountsForPosts(ctx context.Context, postIDs []uint) (map[uint]int64, error) {
	counts, err := s.repo.CountsForPosts(ctx, postIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return counts, nil
}

// errNotYourComment is a 403 rather than a 404-in-disguise, for the same reason
// posts are: every signed-in caller may already read every comment, so the
// row's existence is not a secret and "not yours" is the more useful answer.
func errNotYourComment() *apperror.Error {
	return apperror.Forbidden("You may only modify your own comments.")
}
