package models

// Post belongs to a User. Created/updated timestamps and the soft-delete
// column come from Base, so they are not repeated here.
type Post struct {
	Base

	// UserID is the author. The constraint tag is what makes AutoMigrate emit a
	// real foreign key rather than a bare column that merely looks like one.
	//
	// OnDelete:CASCADE matches the other tables that reference users: a deleted
	// account should not leave rows pointing at an id that no longer exists.
	// Note this is the *hard* delete path — User soft-deletes by default, and a
	// soft-deleted row keeps its id, so cascading only fires on Unscoped deletes.
	UserID uint `gorm:"not null;index" json:"user_id"`

	Title   string `gorm:"size:200;not null;index" json:"title"`
	Content string `gorm:"type:text;not null" json:"content"`

	// User is the belongs-to association. Populated only when a query asks for
	// it (`Preload("User")`), and never serialised on its own — the API shapes
	// output through internal/resources, not through the model.
	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

// TableName pins the table name so refactors of the Go type can't rename it.
func (Post) TableName() string { return "posts" }
