package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/scriptertoufiq/gobook/internal/models"
)

// PostUser identifies one person's reaction to one post — the pair the unique
// index is built on, and the unit the flusher works in.
type PostUser struct {
	PostID uint
	UserID uint
}

// batchSize caps how many rows go into a single statement. Large enough that a
// busy flush is still one or two round trips, small enough to stay well under
// max_allowed_packet and to keep any single lock short.
const batchSize = 500

// ReactionRepository is the durable side of reactions. It is written to by the
// background flusher rather than by request handlers — a request never waits
// on any of this.
type ReactionRepository interface {
	// UpsertBatch inserts reactions, replacing the type where the person has
	// already reacted to that post.
	UpsertBatch(ctx context.Context, reactions []models.Reaction) error

	// DeleteBatch removes reactions that were taken back.
	DeleteBatch(ctx context.Context, pairs []PostUser) error

	// CountsForPost tallies a post's reactions by type. Used to warm the cache
	// for a post nobody has touched since the last restart.
	CountsForPost(ctx context.Context, postID uint) (map[string]int64, error)

	// TypeForUser returns one person's reaction and when it was last written,
	// or "" when they have none. The time matters because a queued offline
	// action older than the stored row must not replace it.
	TypeForUser(ctx context.Context, postID, userID uint) (string, time.Time, error)

	// AllForUser returns every reaction a person has made, keyed by post.
	//
	// Read once per person rather than once per person per post: it is what
	// lets Redis hold only real reactions and still answer "did I react to
	// this?" for posts they merely scrolled past.
	AllForUser(ctx context.Context, userID uint) (map[uint]UserReaction, error)

	// CountsForPosts tallies several posts at once, so warming a page of a
	// feed is one query rather than one per post.
	CountsForPosts(ctx context.Context, postIDs []uint) (map[uint]map[string]int64, error)

	// WithTx returns a repository bound to a transaction, so the flusher can
	// commit its upserts and deletes together.
	WithTx(tx *gorm.DB) ReactionRepository
}

// UserReaction is a stored reaction as the cache needs it.
type UserReaction struct {
	Type      string
	UpdatedAt time.Time
}

type reactionRepository struct {
	db *gorm.DB
}

func NewReactionRepository(db *gorm.DB) ReactionRepository {
	return &reactionRepository{db: db}
}

func (r *reactionRepository) WithTx(tx *gorm.DB) ReactionRepository {
	return &reactionRepository{db: tx}
}

// UpsertBatch is the whole reason the unique index exists. A first-time
// reaction inserts and a changed one updates, so the caller never has to know
// which it was — no read-then-write, no per-row branching.
func (r *reactionRepository) UpsertBatch(ctx context.Context, reactions []models.Reaction) error {
	if len(reactions) == 0 {
		return nil
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "post_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"type", "updated_at"}),
		}).
		CreateInBatches(&reactions, batchSize).Error
	if err != nil {
		return fmt.Errorf("reactions: upsert %d row(s): %w", len(reactions), err)
	}
	return nil
}

// DeleteBatch removes rows by (post_id, user_id) tuple.
//
// A tuple IN keeps this to one statement per chunk. Deleting per row would be
// one round trip each, which defeats the point of batching at all.
func (r *reactionRepository) DeleteBatch(ctx context.Context, pairs []PostUser) error {
	if len(pairs) == 0 {
		return nil
	}

	for start := 0; start < len(pairs); start += batchSize {
		end := min(start+batchSize, len(pairs))
		chunk := pairs[start:end]

		placeholders := make([]string, len(chunk))
		args := make([]any, 0, len(chunk)*2)
		for i, p := range chunk {
			placeholders[i] = "(?,?)"
			args = append(args, p.PostID, p.UserID)
		}

		sql := "DELETE FROM reactions WHERE (post_id, user_id) IN (" +
			strings.Join(placeholders, ",") + ")"

		if err := r.db.WithContext(ctx).Exec(sql, args...).Error; err != nil {
			return fmt.Errorf("reactions: delete %d row(s): %w", len(chunk), err)
		}
	}

	return nil
}

func (r *reactionRepository) AllForUser(ctx context.Context, userID uint) (map[uint]UserReaction, error) {
	var rows []struct {
		PostID    uint
		Type      string
		UpdatedAt time.Time
	}

	err := r.db.WithContext(ctx).
		Model(&models.Reaction{}).
		Select("post_id", "type", "updated_at").
		Where("user_id = ?", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("reactions: read reactions of user %d: %w", userID, err)
	}

	out := make(map[uint]UserReaction, len(rows))
	for _, row := range rows {
		out[row.PostID] = UserReaction{Type: row.Type, UpdatedAt: row.UpdatedAt}
	}
	return out, nil
}

// CountsForPosts groups by post as well as by type, so one query warms a whole
// page of a feed.
func (r *reactionRepository) CountsForPosts(ctx context.Context, postIDs []uint) (map[uint]map[string]int64, error) {
	out := make(map[uint]map[string]int64, len(postIDs))
	if len(postIDs) == 0 {
		return out, nil
	}

	var rows []struct {
		PostID uint
		Type   string
		Total  int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.Reaction{}).
		Select("post_id, type, COUNT(*) AS total").
		Where("post_id IN ?", postIDs).
		Group("post_id, type").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("reactions: count for %d post(s): %w", len(postIDs), err)
	}

	for _, row := range rows {
		if out[row.PostID] == nil {
			out[row.PostID] = map[string]int64{}
		}
		out[row.PostID][row.Type] = row.Total
	}

	// Posts with no reactions still need an entry, or they look unhydrated
	// forever and re-query on every read.
	for _, id := range postIDs {
		if out[id] == nil {
			out[id] = map[string]int64{}
		}
	}

	return out, nil
}

func (r *reactionRepository) CountsForPost(ctx context.Context, postID uint) (map[string]int64, error) {
	var rows []struct {
		Type  string
		Total int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.Reaction{}).
		Select("type, COUNT(*) AS total").
		Where("post_id = ?", postID).
		Group("type").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("reactions: count for post %d: %w", postID, err)
	}

	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.Type] = row.Total
	}
	return counts, nil
}

// TypeForUser returns "" rather than an error when the person has not reacted —
// having no reaction is an ordinary answer, not a fault.
func (r *reactionRepository) TypeForUser(ctx context.Context, postID, userID uint) (string, time.Time, error) {
	var reaction models.Reaction

	err := r.db.WithContext(ctx).
		Select("type", "updated_at").
		Where("post_id = ? AND user_id = ?", postID, userID).
		First(&reaction).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return "", time.Time{}, nil
	case err != nil:
		return "", time.Time{}, fmt.Errorf("reactions: read reaction of user %d on post %d: %w", userID, postID, err)
	}

	return reaction.Type, reaction.UpdatedAt, nil
}
