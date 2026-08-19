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
// has touched since the last restart, and is written only by the background
// flusher. A request never waits on a durable write.
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

	// Before anything increments the tally. A post's hash must carry its stored
	// baseline before a live reaction lands on it, or the count is short by
	// however many reactions the database already held.
	if err := s.ensureHydrated(ctx, postID); err != nil {
		return Summary{}, err
	}

	mine, tally, applied, err := s.store.Set(ctx, postID, userID, reaction, s.stamp(actedAt))
	if err != nil {
		return Summary{}, apperror.Internal(err)
	}

	if mine == reactions.None {
		mine = ""
	}

	// Not applied means a newer reaction was already recorded — the caller is
	// replaying something stale. The tally returned is the current one, so the
	// client can settle on it and stop retrying.
	return Summary{Counts: tally.Counts, Total: tally.Total, Mine: mine, Applied: applied}, nil
}

// stamp normalises when an action happened.
//
// A zero time means "now" — an ordinary online request. A supplied time comes
// from a client replaying its offline queue, and is clamped to the present:
// a device with a wrong clock claiming the future would otherwise win every
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

	if err := s.ensureHydrated(ctx, postID); err != nil {
		return Summary{}, err
	}

	current, known, err := s.store.Mine(ctx, postID, userID)
	if err != nil {
		return Summary{}, apperror.Internal(err)
	}

	if !known {
		// Not loaded yet — the answer is in MySQL.
		current, err = s.hydrateMine(ctx, postID, userID)
		if err != nil {
			return Summary{}, err
		}
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
	// saying the reaction is gone when it is not would have the client render
	// a state the server does not hold.
	if mine == reactions.None {
		mine = ""
	}

	return Summary{Counts: tally.Counts, Total: tally.Total, Mine: mine, Applied: applied}, nil
}

// Summary reads a post's tally and the viewer's own choice, warming whichever
// is missing from the database exactly once.
//
// Pass 0 for viewerID to read the tally without a personal answer.
func (s *ReactionService) Summary(ctx context.Context, postID, viewerID uint) (Summary, error) {
	if err := s.ensureHydrated(ctx, postID); err != nil {
		return Summary{}, err
	}

	// Read from Redis, never from the hydration result. Redis is authoritative
	// here — the database lags behind by up to a flush interval — so returning
	// what MySQL held would report a stale tally, and would report an empty one
	// for any post whose reactions have not been flushed yet.
	tally, _, err := s.store.Counts(ctx, postID)
	if err != nil {
		return Summary{}, apperror.Internal(err)
	}

	summary := Summary{Counts: tally.Counts, Total: tally.Total, Applied: true}
	if viewerID == 0 {
		return summary, nil
	}

	mine, known, err := s.store.Mine(ctx, postID, viewerID)
	if err != nil {
		return Summary{}, apperror.Internal(err)
	}
	if !known {
		mine, err = s.hydrateMine(ctx, postID, viewerID)
		if err != nil {
			return Summary{}, err
		}
	}

	summary.Mine = mine
	return summary, nil
}

// ensureHydrated guarantees a post's tally carries its stored baseline before
// anything reads or increments it.
//
// Runs once per post per Redis lifetime: the marker it sets is what makes every
// later call a single EXISTS. The store applies the baseline atomically, so two
// requests racing here cannot lose a reaction between them.
func (s *ReactionService) ensureHydrated(ctx context.Context, postID uint) error {
	_, hydrated, err := s.store.Counts(ctx, postID)
	if err != nil {
		return apperror.Internal(err)
	}
	if hydrated {
		return nil
	}

	counts, err := s.repo.CountsForPost(ctx, postID)
	if err != nil {
		return apperror.Internal(err)
	}

	if err := s.store.Hydrate(ctx, postID, counts); err != nil {
		// Not fatal: the tally Redis already holds is still the live one, and
		// the next request will try again.
		log.Printf("reactions: could not hydrate post %d: %v", postID, err)
	}

	return nil
}

// hydrateMine reads one person's stored reaction and caches it — including the
// fact that they have none, which is what stops the lookup repeating on every
// page view by a non-reactor.
func (s *ReactionService) hydrateMine(ctx context.Context, postID, userID uint) (string, error) {
	stored, storedAt, err := s.repo.TypeForUser(ctx, postID, userID)
	if err != nil {
		return "", apperror.Internal(err)
	}

	if err := s.store.RememberMine(ctx, postID, userID, stored, storedAt); err != nil {
		log.Printf("reactions: could not cache reaction of user %d on post %d: %v", userID, postID, err)
	}

	return stored, nil
}

// SummariseMany reads tallies for a page of posts.
//
// Sequential rather than pipelined, because each post may need hydrating from
// a different set of rows and that cannot be batched into one Redis call. It
// is still cheap — a hydrated post costs two Redis reads — and it only touches
// the database for posts nobody has looked at since the last restart.
//
// A failure on one post yields an empty tally for it rather than failing the
// whole page: a feed that renders without counts beats a feed that does not
// render.
func (s *ReactionService) SummariseMany(
	ctx context.Context,
	postIDs []uint,
	viewerID uint,
) map[uint]Summary {
	out := make(map[uint]Summary, len(postIDs))

	for _, id := range postIDs {
		summary, err := s.Summary(ctx, id, viewerID)
		if err != nil {
			log.Printf("reactions: could not summarise post %d for a listing: %v", id, err)
			continue
		}
		out[id] = summary
	}

	return out
}

// Forget clears a deleted post's cached tally. Its rows go with the post
// through the foreign key, so the cache must not outlive them.
func (s *ReactionService) Forget(ctx context.Context, postID uint) error {
	return s.store.Forget(ctx, postID)
}
