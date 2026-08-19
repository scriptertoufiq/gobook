package services

import (
	"context"
	"log"
	"time"

	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/internal/reactions"
	"github.com/scriptertoufiq/gobook/internal/repositories"
	"github.com/scriptertoufiq/gobook/pkg/apperror"
)

// ReactionService owns the reaction rules. Like every service here it knows
// nothing about HTTP.
//
// Reads and writes go to Redis; MySQL is consulted only to warm a post nobody
// has touched since the last restart, or a person whose reactions are not
// loaded yet, and is written only by the background flusher. A request never
// waits on a durable write.
type ReactionService struct {
	store *reactions.Store
	repo  repositories.ReactionRepository
	posts *PostService
}

func NewReactionService(
	store *reactions.Store,
	repo repositories.ReactionRepository,
	posts *PostService,
) *ReactionService {
	return &ReactionService{store: store, repo: repo, posts: posts}
}

// Summary is what a caller learns about a post's reactions.
type Summary struct {
	Counts map[string]int64
	Total  int64
	// Mine is the viewer's own reaction, or "" when they have none.
	Mine string

	// Applied is false when a replayed action was discarded for being older
	// than what is already stored. Reads leave it true — nothing was rejected.
	Applied bool
}

// Set applies a reaction. Sending the one already held takes it back, which is
// how every reaction UI behaves and the only way to undo without a second
// control.
func (s *ReactionService) Set(
	ctx context.Context,
	postID, userID uint,
	reaction string,
	actedAt time.Time,
) (Summary, error) {
	if !models.IsValidReaction(reaction) {
		return Summary{}, apperror.Validation("That is not a reaction this app accepts.")
	}

	// Reacting to a post that does not exist would strand a row the foreign key
	// then rejects at flush time — by which point nobody is around to be told.
	if _, _, err := s.posts.Get(ctx, postID); err != nil {
		return Summary{}, err
	}

	// Both baselines must be in place before anything is incremented: the
	// post's tally, or the count is short by whatever the database already
	// held, and this person's set, or their existing reaction is invisible and
	// gets double-counted.
	if err := s.warmPosts(ctx, []uint{postID}); err != nil {
		return Summary{}, err
	}
	if err := s.warmUser(ctx, userID); err != nil {
		return Summary{}, err
	}

	mine, tally, applied, err := s.store.Set(ctx, postID, userID, reaction, s.stamp(actedAt))
	if err != nil {
		return Summary{}, apperror.Internal(err)
	}

	if mine == reactions.None {
		mine = ""
	}

	return Summary{Counts: tally.Counts, Total: tally.Total, Mine: mine, Applied: applied}, nil
}

// stamp normalises when an action happened.
//
// A zero time means "now" — an ordinary online request. A supplied time comes
// from a client replaying its offline queue, and is clamped to the present: a
// device with a wrong clock claiming the future would otherwise win every
// conflict for as long as its clock stayed ahead.
func (s *ReactionService) stamp(actedAt time.Time) time.Time {
	now := time.Now()
	if actedAt.IsZero() || actedAt.After(now) {
		return now
	}
	return actedAt
}

// Remove takes a reaction back. Idempotent: removing when nothing is held is a
// success, because the caller's intent is already satisfied.
func (s *ReactionService) Remove(
	ctx context.Context,
	postID, userID uint,
	actedAt time.Time,
) (Summary, error) {
	if _, _, err := s.posts.Get(ctx, postID); err != nil {
		return Summary{}, err
	}
	if err := s.warmPosts(ctx, []uint{postID}); err != nil {
		return Summary{}, err
	}
	if err := s.warmUser(ctx, userID); err != nil {
		return Summary{}, err
	}

	current, _, err := s.store.Mine(ctx, postID, userID)
	if err != nil {
		return Summary{}, apperror.Internal(err)
	}

	if current == "" {
		// Nothing to take back. Report the tally as it stands rather than
		// writing a no-op through the script and into the pending set.
		return s.Summary(ctx, postID, userID)
	}

	// Sending the held reaction back through Set is what clears it.
	mine, tally, applied, err := s.store.Set(ctx, postID, userID, current, s.stamp(actedAt))
	if err != nil {
		return Summary{}, apperror.Internal(err)
	}

	// Report what the script left in place rather than assuming the removal
	// took. A replayed removal older than the stored reaction is rejected, and
	// saying the reaction is gone when it is not would have the client render a
	// state the server does not hold.
	if mine == reactions.None {
		mine = ""
	}

	return Summary{Counts: tally.Counts, Total: tally.Total, Mine: mine, Applied: applied}, nil
}

// Summary reads one post's tally and the viewer's own choice.
//
// Pass 0 for viewerID to read the tally without a personal answer.
func (s *ReactionService) Summary(ctx context.Context, postID, viewerID uint) (Summary, error) {
	all, err := s.SummariseMany(ctx, []uint{postID}, viewerID)
	if err != nil {
		return Summary{}, err
	}

	summary, ok := all[postID]
	if !ok {
		return Summary{Counts: map[string]int64{}, Applied: true}, nil
	}
	return summary, nil
}

// SummariseMany reads tallies for a page of posts.
//
// The cost is fixed rather than per-post: one pipeline for every tally, one
// HMGET for the viewer's own reactions, and at most one database query for
// whatever is not warm yet. The earlier version looped post by post, which on
// a ten-post feed meant twenty sequential round trips and up to ten separate
// queries.
func (s *ReactionService) SummariseMany(
	ctx context.Context,
	postIDs []uint,
	viewerID uint,
) (map[uint]Summary, error) {
	out := make(map[uint]Summary, len(postIDs))
	if len(postIDs) == 0 {
		return out, nil
	}

	if err := s.warmPosts(ctx, postIDs); err != nil {
		return nil, err
	}
	if err := s.warmUser(ctx, viewerID); err != nil {
		return nil, err
	}

	tallies, _, err := s.store.CountsMany(ctx, postIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	mine := map[uint]string{}
	if viewerID != 0 {
		mine, _, err = s.store.MineMany(ctx, postIDs, viewerID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
	}

	for _, id := range postIDs {
		tally := tallies[id]
		out[id] = Summary{
			Counts:  tally.Counts,
			Total:   tally.Total,
			Mine:    mine[id],
			Applied: true,
		}
	}

	return out, nil
}

// warmPosts loads the stored baseline for any of these posts that lacks one.
//
// One query for the whole page, not one per post. Hydration happens once per
// post per Redis lifetime; after that this costs a single pipelined EXISTS each.
func (s *ReactionService) warmPosts(ctx context.Context, postIDs []uint) error {
	_, cold, err := s.store.CountsMany(ctx, postIDs)
	if err != nil {
		return apperror.Internal(err)
	}
	if len(cold) == 0 {
		return nil
	}

	counts, err := s.repo.CountsForPosts(ctx, cold)
	if err != nil {
		return apperror.Internal(err)
	}

	for _, id := range cold {
		if err := s.store.Hydrate(ctx, id, counts[id]); err != nil {
			// The tally Redis already holds is still the live one; the next
			// read will try again rather than the request failing.
			log.Printf("reactions: could not hydrate post %d: %v", id, err)
		}
	}

	return nil
}

// warmUser loads a person's complete set of reactions, once.
//
// This is what allows Redis to store nothing at all for the posts somebody
// merely scrolled past: with their full set present, a missing field is a
// definite "they did not react" rather than "we have not checked".
func (s *ReactionService) warmUser(ctx context.Context, userID uint) error {
	if userID == 0 {
		return nil
	}

	loaded, err := s.store.UserLoaded(ctx, userID)
	if err != nil {
		return apperror.Internal(err)
	}
	if loaded {
		return nil
	}

	stored, err := s.repo.AllForUser(ctx, userID)
	if err != nil {
		return apperror.Internal(err)
	}

	converted := make(map[uint]reactions.StoredReaction, len(stored))
	for postID, r := range stored {
		converted[postID] = reactions.StoredReaction{Type: r.Type, At: r.UpdatedAt}
	}

	if err := s.store.LoadUser(ctx, userID, converted); err != nil {
		log.Printf("reactions: could not load reactions of user %d: %v", userID, err)
	}

	return nil
}

// Forget clears a deleted post's cached tally. Its rows go with the post
// through the foreign key, so the cache must not outlive them.
func (s *ReactionService) Forget(ctx context.Context, postID uint) error {
	return s.store.Forget(ctx, postID)
}
