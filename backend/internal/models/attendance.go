package models

import (
	"time"
)

// Attendance represents the event check-in record for a participant on event day.
type Attendance struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ParticipantID uint      `json:"participant_id" gorm:"not null;uniqueIndex:idx_part_event"`
	EventID       uint      `json:"event_id" gorm:"not null;uniqueIndex:idx_part_event"`
	CheckedInAt   time.Time `json:"checked_in_at" gorm:"not null;index"`
	CheckedInBy   string    `json:"checked_in_by" gorm:"size:255"`
	CreatedAt     time.Time `json:"created_at"`

	// Associations
	Participant Participant `json:"participant,omitempty" gorm:"foreignKey:ParticipantID;constraint:OnDelete:CASCADE"`
	Event       Event       `json:"event,omitempty" gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE"`
}
