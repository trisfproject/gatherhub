package models

import (
	"time"
)

// Event represents an event that participants can register for
type Event struct {
	ID                   uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Title                string    `json:"title" gorm:"not null;size:255"`
	Description          string    `json:"description" gorm:"type:text"`
	EventDate            time.Time `json:"event_date" gorm:"not null"`
	Location             string    `json:"location" gorm:"not null;size:500"`
	Price                float64   `json:"price" gorm:"not null;default:0"`
	PaymentBank          string    `json:"payment_bank" gorm:"size:100"`
	PaymentAccountNumber string    `json:"payment_account_number" gorm:"size:50"`
	PaymentAccountName   string    `json:"payment_account_name" gorm:"size:255"`
	AdminName            string    `json:"admin_name" gorm:"not null;size:255"`
	AdminWhatsapp        string    `json:"admin_whatsapp" gorm:"not null;size:20"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`

	// Associations
	Participants []Participant `json:"participants,omitempty" gorm:"foreignKey:EventID"`
}
