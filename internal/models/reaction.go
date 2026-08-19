package models

import "time"

// Reaction is one person's response to one post.
//
// The unique index on (post_id, user_id) is what the whole write-behind design
// leans on: the flusher can INSERT ... ON DUPLICATE KEY UPDATE a whole batch
// without knowing which rows already exist.
//
// Note what is missing — there is no soft-delete column, and that is
// deliberate. Taking a reaction back leaves nothing worth keeping, and a
// soft-deleted row would keep occupying the unique index, so the same person
// could never react to that post again. Same reasoning that keeps the
// migration ledger off Base.
type Reaction struct {
	Base

	PostID uint   `gorm:"not null;uniqueIndex:idx_reactions_post_user;index" json:"post_id"`
	UserID uint   `gorm:"not null;uniqueIndex:idx_reactions_post_user" json:"user_id"`
	Type   string `gorm:"size:16;not null" json:"type"`

	Post *Post `gorm:"foreignKey:PostID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`

	// Declared here rather than through Timestamps because that embed carries
	// DeletedAt. Keeping the house convention of bookkeeping columns last.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Reaction) TableName() string { return "reactions" }

// The five responses a post accepts.
const (
	ReactionLike  = "like"
	ReactionLove  = "love"
	ReactionCare  = "care"
	ReactionSad   = "sad"
	ReactionAngry = "angry"
)

// ReactionTypes is the canonical order — the order a picker shows them in, and
// the order counts are reported in.
var ReactionTypes = []string{
	ReactionLike, ReactionLove, ReactionCare, ReactionSad, ReactionAngry,
}

// IsValidReaction reports whether a type is one this app accepts. Callers
// validate before storing, because the cache layer will happily key a hash
// field on whatever string it is handed.
func IsValidReaction(t string) bool {
	for _, valid := range ReactionTypes {
		if t == valid {
			return true
		}
	}
	return false
}
