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
	IndustrialEstateName string            `json:"industrial_estate_name" gorm:"size:255"`
	TelegramUsername   string            `json:"telegram_username" gorm:"size:100"`
	JobTitle           *string           `json:"job_title" gorm:"size:255"`
	EmergencyName      string            `json:"emergency_name" gorm:"size:255"`
	EmergencyRelationship string         `json:"emergency_relationship" gorm:"size:100"`
	EmergencyPhone     string            `json:"emergency_phone" gorm:"size:20"`
	OwnVehicle         bool              `json:"own_vehicle" gorm:"default:false"`
	VehicleType        string            `json:"vehicle_type" gorm:"size:50"`
	LicensePlate       string            `json:"license_plate" gorm:"size:50"`
	CarpoolCanBring    bool              `json:"carpool_can_bring" gorm:"default:false"`
	CarpoolSeats       int               `json:"carpool_seats" gorm:"default:0"`
	TShirtSize         string            `json:"tshirt_size" gorm:"size:10"`
	PaymentProof       string            `json:"payment_proof" gorm:"size:500"`
	Status             ParticipantStatus `json:"status" gorm:"not null;default:PENDING;size:20;index;index:idx_event_status"`
	VerifiedAt         *time.Time        `json:"verified_at,omitempty"`
	RejectedAt         *time.Time        `json:"rejected_at,omitempty"`
	DepartureZone      string            `json:"departure_zone" gorm:"size:255"`
	DepartureZoneName  string            `json:"departure_zone_name" gorm:"size:255"`
	DriverID           *uint             `json:"driver_id" gorm:"index"`
	TransportMeetingPoint string         `json:"transport_meeting_point" gorm:"size:255"`
	TransportDepartureTime string        `json:"transport_departure_time" gorm:"size:255"`
	TransportNotes     string            `json:"transport_notes" gorm:"type:text"`
	OfficialDriver     bool              `json:"official_driver" gorm:"default:false"`
	CreatedAt          time.Time         `json:"created_at" gorm:"index"`
	UpdatedAt          time.Time         `json:"updated_at"`

	// Associations
	Event Event `json:"event,omitempty" gorm:"foreignKey:EventID"`
	Driver *Participant `json:"driver,omitempty" gorm:"foreignKey:DriverID;references:ID"`
}

// GetTransportationStatus returns the active transportation role/status
func (p *Participant) GetTransportationStatus() string {
	if p.OfficialDriver {
		return "Official Driver"
	}
	if p.OwnVehicle && (p.VehicleType == "Car" || p.VehicleType == "Mobil") && p.CarpoolCanBring {
		return "Volunteer Driver"
	}
	if p.DriverID != nil {
		return "Passenger"
	}
	return "Not Participating"
}
