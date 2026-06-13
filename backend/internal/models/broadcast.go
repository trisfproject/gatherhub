package models

import (
	"time"
)

// Broadcast represents a bulk announcement sent to a filtered group of participants
type Broadcast struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	EventID         uint      `json:"event_id" gorm:"not null;index"`
	Title           string    `json:"title" gorm:"not null;size:255"`
	Message         string    `json:"message" gorm:"type:text;not null"`
	Audience        string    `json:"audience" gorm:"not null;size:255"` // Description of target, e.g., "Verified in Bandung"
	TotalRecipients int       `json:"total_recipients" gorm:"not null;default:0"`
	SentCount       int       `json:"sent_count" gorm:"not null;default:0"`
	FailedCount     int       `json:"failed_count" gorm:"not null;default:0"`
	CreatedAt       time.Time `json:"created_at"`

	// Associations
	Event Event `json:"event,omitempty" gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE"`
}
