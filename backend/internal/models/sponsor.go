package models

import (
	"time"
)

// Sponsor categories
const (
	CategoryTitleSponsor     = "Title Sponsor"
	CategoryPlatinumSponsor  = "Platinum Sponsor"
	CategoryGoldSponsor      = "Gold Sponsor"
	CategorySilverSponsor    = "Silver Sponsor"
	CategoryBronzeSponsor    = "Bronze Sponsor"
	CategoryCommunityPartner = "Community Partner"
	CategoryMediaPartner     = "Media Partner"
)

// Sponsor represents a sponsor or partner associated with an event.
type Sponsor struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	EventID      uint      `json:"event_id" gorm:"not null"`
	Name         string    `json:"name" gorm:"not null;size:255"`
	Category     string    `json:"category" gorm:"not null;size:100"` // Title Sponsor, Platinum Sponsor, etc.
	Logo         string    `json:"logo" gorm:"not null;size:255"`
	WebsiteURL   string    `json:"website_url" gorm:"size:500"`
	DisplayOrder int       `json:"display_order" gorm:"not null;default:0"`
	Active       bool      `json:"active" gorm:"not null;default:true"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Associations
	Event Event `json:"event,omitempty" gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE"`
}
