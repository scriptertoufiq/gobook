package models

import (
	"time"

	"gorm.io/gorm"
)

// Base carries the columns every table shares. Embed it in each model
// instead of gorm.Model so we control the JSON tags.
type Base struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // presence of this field enables soft deletes
}
