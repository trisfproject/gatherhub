package models

import (
	"time"
)

// ParticipantStatus represents the registration approval state
type ParticipantStatus string

const (
	StatusPending   ParticipantStatus = "PENDING"
	StatusVerified  ParticipantStatus = "VERIFIED"
	StatusRejected  ParticipantStatus = "REJECTED"
	StatusCheckedIn ParticipantStatus = "CHECKED_IN"
)

// Participant represents a person who registered for an event
type Participant struct {
	ID                 uint              `json:"id" gorm:"primaryKey;autoIncrement"`
	EventID            uint              `json:"event_id" gorm:"not null;index;index:idx_event_status"`
	RegistrationNumber string            `json:"registration_number" gorm:"uniqueIndex;size:20"`
	FullName           string            `json:"full_name" gorm:"not null;size:255"`
	Phone              string            `json:"phone" gorm:"not null;size:20"`
	Email              string            `json:"email" gorm:"not null;size:255"`
	City               string            `json:"city" gorm:"not null;size:100"`
	CompanyName        string            `json:"company_name" gorm:"size:255"`
	IndustrialEstate   string            `json:"industrial_estate" gorm:"size:255"`
	TelegramUsername   string            `json:"telegram_username" gorm:"size:100"`
	JobTitle           *string           `json:"job_title" gorm:"size:255"`
	PaymentProof       string            `json:"payment_proof" gorm:"size:500"`
	Status             ParticipantStatus `json:"status" gorm:"not null;default:PENDING;size:20;index;index:idx_event_status"`
	VerifiedAt         *time.Time        `json:"verified_at,omitempty"`
	RejectedAt         *time.Time        `json:"rejected_at,omitempty"`
	CreatedAt          time.Time         `json:"created_at" gorm:"index"`
	UpdatedAt          time.Time         `json:"updated_at"`

	// Associations
	Event Event `json:"event,omitempty" gorm:"foreignKey:EventID"`
}
