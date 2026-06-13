package models

import "time"

// Setting represents a system configuration key-value pair.
type Setting struct {
	Key       string    `json:"key" gorm:"primaryKey;size:100"`
	Value     string    `json:"value" gorm:"type:text;not null"`
	Category  string    `json:"category" gorm:"size:50;not null;index"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
