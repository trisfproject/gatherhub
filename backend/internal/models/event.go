package models

import (
	"time"
)

// Event represents an event that participants can register for
type Event struct {
	ID                   uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Title                string    `json:"title" gorm:"not null;size:255"`
	Slug                 string    `json:"slug" gorm:"uniqueIndex;not null;size:255"`
	Description          string    `json:"description" gorm:"type:text"`
	BannerImage          string    `json:"banner_image" gorm:"size:500"`
	EventDate            time.Time `json:"event_date" gorm:"not null"`
	EventTime            string    `json:"event_time" gorm:"size:50"`
	Location             string    `json:"location" gorm:"not null;size:500"`
	Price                float64   `json:"price" gorm:"not null;default:0"`
	PaymentBank          string    `json:"payment_bank" gorm:"size:100"`
	PaymentAccountNumber string    `json:"payment_account_number" gorm:"size:50"`
	PaymentAccountName   string    `json:"payment_account_name" gorm:"size:255"`
	AdminName            string    `json:"admin_name" gorm:"not null;size:255"`
	AdminWhatsapp        string    `json:"admin_whatsapp" gorm:"not null;size:20"`
	MaxParticipants      int       `json:"max_participants" gorm:"not null;default:0"`
	RegistrationOpen     time.Time `json:"registration_open"`
	RegistrationClose    time.Time `json:"registration_close"`
	Status               string    `json:"status" gorm:"not null;default:'DRAFT';size:20"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`

	// Associations
	Participants []Participant `json:"participants,omitempty" gorm:"foreignKey:EventID"`
}
