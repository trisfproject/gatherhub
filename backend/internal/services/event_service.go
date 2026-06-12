package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gatherhub/backend/internal/models"
	"gorm.io/gorm"
)

// EventService handles event business logic
type EventService struct {
	db *gorm.DB
}

// NewEventService creates a new EventService
func NewEventService(db *gorm.DB) *EventService {
	return &EventService{db: db}
}

// GetAll returns all events ordered by event date ascending
func (s *EventService) GetAll() ([]models.Event, error) {
	var events []models.Event
	result := s.db.Order("event_date ASC").Find(&events)
	return events, result.Error
}

// GetByID returns a single event by ID, preloading its participants
func (s *EventService) GetByID(id uint) (*models.Event, error) {
	var event models.Event
	result := s.db.Preload("Participants").First(&event, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("event with id %d not found", id)
	}
	return &event, result.Error
}

// GetBySlug returns a single event by Slug
func (s *EventService) GetBySlug(slug string) (*models.Event, error) {
	var event models.Event
	result := s.db.Where("slug = ?", slug).First(&event)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("event with slug %s not found", slug)
	}
	return &event, result.Error
}

// GetFirst returns the first active published event (earliest by created_at)
// This is used for the single-event landing page flow
func (s *EventService) GetFirst() (*models.Event, error) {
	var event models.Event
	result := s.db.Where("status = ?", "PUBLISHED").Order("created_at ASC").First(&event)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("no published events found")
	}
	return &event, result.Error
}

// Create saves a new event to the database.
// If the new event is PUBLISHED, all other currently-published events are set to CLOSED.
func (s *EventService) Create(event *models.Event) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if event.Status == "PUBLISHED" {
			if err := tx.Model(&models.Event{}).Where("status = ?", "PUBLISHED").Update("status", "CLOSED").Error; err != nil {
				return err
			}
		}
		return tx.Create(event).Error
	})
}

// Update updates an existing event in the database.
// If the updated event is PUBLISHED, all other currently-published events are set to CLOSED.
func (s *EventService) Update(event *models.Event) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if event.Status == "PUBLISHED" {
			if err := tx.Model(&models.Event{}).Where("id != ? AND status = ?", event.ID, "PUBLISHED").Update("status", "CLOSED").Error; err != nil {
				return err
			}
		}
		return tx.Save(event).Error
	})
}

// Delete removes an event and all its associated participants
func (s *EventService) Delete(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete participants first to satisfy foreign key constraints
		if err := tx.Where("event_id = ?", id).Delete(&models.Participant{}).Error; err != nil {
			return err
		}
		// Delete event
		if err := tx.Delete(&models.Event{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

// SeedSampleIfEmpty creates a demo event if the events table is empty
func (s *EventService) SeedSampleIfEmpty() error {
	var count int64
	s.db.Model(&models.Event{}).Count(&count)
	if count > 0 {
		return nil
	}

	eventDate, _ := time.Parse("2006-01-02 15:04:05", "2025-08-15 09:00:00")

	sample := models.Event{
		Title: "Industrial Technology Summit 2025",
		Slug:  "industrial-technology-summit-2025",
		Description: `Bergabunglah bersama kami dalam acara pertemuan teknologi industri terbesar tahun ini. Jalin jaringan dengan para pemimpin industri, jelajahi solusi otomasi terkini, dan dapatkan wawasan mendalam tentang masa depan manufaktur dan kawasan industri di Asia Tenggara.

Topik yang akan dibahas:
• Smart Factory & Industry 4.0
• Integrasi IoT dalam Manufaktur
• Praktik Industri Berkelanjutan
• Studi Kasus Transformasi Digital
• Keamanan dan Keselamatan Industri`,
		EventDate:            eventDate,
		EventTime:            "09:00 - 17:00",
		Location:             "Jakarta Convention Center, Hall A, Jakarta Pusat",
		Price:                350000,
		PaymentBank:          "Bank Central Asia (BCA)",
		PaymentAccountNumber: "1234567890",
		PaymentAccountName:   "PT GatherHub Indonesia",
		AdminName:            "Budi Santoso",
		AdminWhatsapp:        "6281234567890",
		MaxParticipants:      100,
		RegistrationOpen:     time.Now().Add(-24 * time.Hour),
		RegistrationClose:    time.Now().Add(30 * 24 * time.Hour),
		Status:               "PUBLISHED",
	}

	return s.db.Create(&sample).Error
}
