package repositories

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/scriptertoufiq/gobook/internal/models"
	"github.com/scriptertoufiq/gobook/pkg/pagination"
)

// CommentRepository is the contract the service depends on.
type CommentRepository interface {
	Create(ctx context.Context, comment *models.Comment) error
	Update(ctx context.Context, comment *models.Comment) error

	// DeleteWithReplies removes a comment and everything hanging off it.
	//
	// The foreign key cascade does not cover this: it fires on a hard delete,
	// and comments soft-delete like posts do. Without removing the replies
	// explicitly they survive as rows whose parent is gone — invisible in every
	// listing, yet still counted.
	DeleteWithReplies(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*models.Comment, error)

	// PaginateForPost lists a post's top-level comments, oldest first — the
	// order a conversation is read in.
	PaginateForPost(ctx context.Context, postID uint, p pagination.Params) ([]models.Comment, int64, error)

	// PaginateReplies lists the replies under one comment.
	PaginateReplies(ctx context.Context, parentID uint, p pagination.Params) ([]models.Comment, int64, error)

	// ReplyCounts returns how many replies each of these comments has, so a
	// listing can offer "view 3 replies" without a query per row.
	ReplyCounts(ctx context.Context, commentIDs []uint) (map[uint]int64, error)

	// CountsForPosts totals the comments on each post, replies included. One
	// query for a whole page of a feed rather than one per post.
	CountsForPosts(ctx context.Context, postIDs []uint) (map[uint]int64, error)
}

type commentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &commentRepository{db: db}
}

func (r *commentRepository) Create(ctx context.Context, comment *models.Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *commentRepository) Update(ctx context.Context, comment *models.Comment) error {
	return r.db.WithContext(ctx).Save(comment).Error
}

func (r *commentRepository) DeleteWithReplies(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Replies first: if this half succeeds and the next fails, the thread is
		// merely empty rather than orphaned.
		if err := tx.Where("parent_id = ?", id).Delete(&models.Comment{}).Error; err != nil {
			return fmt.Errorf("comments: delete replies of %d: %w", id, err)
		}

		if err := tx.Delete(&models.Comment{}, id).Error; err != nil {
			return fmt.Errorf("comments: delete %d: %w", id, err)
		}
		return nil
	})
}

func (r *commentRepository) FindByID(ctx context.Context, id uint) (*models.Comment, error) {
	var comment models.Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

// PaginateForPost reads the top level only — replies are fetched per thread, so
// a post with one enormous argument under it does not make the first page huge.
//
// Oldest first, unlike posts. A feed shows the newest thing first; a
// conversation is read in the order it happened.
func (r *commentRepository) PaginateForPost(
	ctx context.Context,
	postID uint,
	p pagination.Params,
) ([]models.Comment, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&models.Comment{}).
		Where("post_id = ? AND parent_id IS NULL", postID)

	return r.paginate(query, p)
}

func (r *commentRepository) PaginateReplies(
	ctx context.Context,
	parentID uint,
	p pagination.Params,
) ([]models.Comment, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&models.Comment{}).
		Where("parent_id = ?", parentID)

	return r.paginate(query, p)
}

// commentSortable whitelists what a client may order by. `created_at` is first,
// so it is also the fallback for anything unrecognised.
var commentSortable = []string{"created_at", "id"}

func (r *commentRepository) paginate(query *gorm.DB, p pagination.Params) ([]models.Comment, int64, error) {
	if p.Search != "" {
		query = query.Where("body LIKE ?", "%"+p.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("comments: count: %w", err)
	}

	// Ascending unless asked otherwise — the composite index is (post_id,
	// created_at), so reading in that order needs no sort at all.
	direction := p
	if direction.SortDir == "" {
		direction.SortDir = "asc"
	}

	var comments []models.Comment
	err := query.
		Order(direction.OrderClause(commentSortable, "created_at")).
		Limit(p.PerPage).
		Offset(p.Offset()).
		Find(&comments).Error
	if err != nil {
		return nil, 0, fmt.Errorf("comments: read page: %w", err)
	}

	return comments, total, nil
}

func (r *commentRepository) ReplyCounts(ctx context.Context, commentIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(commentIDs))
	if len(commentIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		ParentID uint
		Total    int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.Comment{}).
		Select("parent_id, COUNT(*) AS total").
		Where("parent_id IN ?", commentIDs).
		Group("parent_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("comments: count replies: %w", err)
	}

	for _, row := range rows {
		counts[row.ParentID] = row.Total
	}

	// Comments with no replies still need an entry, so a caller can tell
	// "none" from "not looked up".
	for _, id := range commentIDs {
		if _, ok := counts[id]; !ok {
			counts[id] = 0
		}
	}

	return counts, nil
}

// CountsForPosts counts both levels: what a post shows is how much conversation
// it has, and a reply is part of that.
func (r *commentRepository) CountsForPosts(ctx context.Context, postIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(postIDs))
	if len(postIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		PostID uint
		Total  int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.Comment{}).
		Select("post_id, COUNT(*) AS total").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("comments: count for %d post(s): %w", len(postIDs), err)
	}

	for _, row := range rows {
		counts[row.PostID] = row.Total
	}
	for _, id := range postIDs {
		if _, ok := counts[id]; !ok {
			counts[id] = 0
		}
	}

	return counts, nil
}
