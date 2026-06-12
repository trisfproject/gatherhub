package models

import (
	"time"
)

// NotificationLog represents a logged message sent via any notification channel
type NotificationLog struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ParticipantID uint      `json:"participant_id" gorm:"not null;index"`
	EventID       uint      `json:"event_id" gorm:"not null;index"`
	Recipient     string    `json:"recipient" gorm:"not null;size:255"` // Phone number, email address, username, etc.
	Channel       string    `json:"channel" gorm:"not null;size:50"`    // WHATSAPP, EMAIL, TELEGRAM, WEBHOOK
	Message       string    `json:"message" gorm:"type:text;not null"`
	Status        string    `json:"status" gorm:"not null;size:50"` // SUCCESS, FAILED
	CreatedAt     time.Time `json:"created_at"`

	// Associations
	Participant Participant `json:"participant,omitempty" gorm:"foreignKey:ParticipantID;constraint:OnDelete:CASCADE"`
	Event       Event       `json:"event,omitempty" gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE"`
}
