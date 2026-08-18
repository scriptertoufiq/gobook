package models

import (
	"time"

	"gorm.io/gorm"
)

// Base carries the primary key. Embed it FIRST — a generated table's column
// order follows struct field order, and `id` belongs at the front.
type Base struct {
	ID uint `gorm:"primaryKey" json:"id"`
}

// Timestamps carries the bookkeeping columns every table shares. Embed it LAST,
// after the fields that actually describe the row, so a `DESCRIBE` reads
// domain-first instead of opening with three columns nobody came to look at.
//
// It is separate from Base purely to make that ordering possible: a single
// embedded struct can only sit in one place, and `id` and the timestamps want
// opposite ends.
type Timestamps struct {
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // presence of this field enables soft deletes
}
