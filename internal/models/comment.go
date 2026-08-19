package models

import (
	"time"

	"gorm.io/gorm"
)

// Comment is something somebody wrote under a post, or under another comment.
//
// Threading is two levels deep on purpose. ParentID is nil for a comment on a
// post, and set for a reply — and a reply's parent must itself be top level,
// which the service enforces. Arbitrary nesting reads well in a diagram and
// badly everywhere else: fetching a tree needs recursive queries, rendering it
// needs indentation nobody wants past the third level, and moderating it means
// reasoning about orphaned sub-threads. Two levels is what people actually use.
type Comment struct {
	Base

	PostID uint `gorm:"not null;index:idx_comments_post_created,priority:1" json:"post_id"`
	UserID uint `gorm:"not null;index" json:"user_id"`

	// ParentID is nil for a top-level comment.
	ParentID *uint `gorm:"index:idx_comments_parent_created,priority:1" json:"parent_id,omitempty"`

	Body string `gorm:"type:text;not null" json:"body"`

	Post   *Post    `gorm:"foreignKey:PostID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	User   *User    `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Parent *Comment `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`

	// CreatedAt is declared here rather than coming from Timestamps so it can
	// join the composite indexes above: a thread is always read in time order
	// within one post or one parent, and a filter column without its sort
	// column still costs a filesort.
	CreatedAt time.Time      `gorm:"index:idx_comments_post_created,priority:2;index:idx_comments_parent_created,priority:2" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Comment) TableName() string { return "comments" }

// IsReply reports whether this comment hangs off another comment.
func (c Comment) IsReply() bool { return c.ParentID != nil }
