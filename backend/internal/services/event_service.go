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

// GetByID returns a single event by ID
func (s *EventService) GetByID(id uint) (*models.Event, error) {
	var event models.Event
	result := s.db.First(&event, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("event with id %d not found", id)
	}
	return &event, result.Error
}

// GetFirst returns the first active event (earliest by created_at)
// This is used for the single-event landing page flow
func (s *EventService) GetFirst() (*models.Event, error) {
	var event models.Event
	result := s.db.Order("created_at ASC").First(&event)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("no events found")
	}
	return &event, result.Error
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
		Description: `Bergabunglah bersama kami dalam acara pertemuan teknologi industri terbesar tahun ini. Jalin jaringan dengan para pemimpin industri, jelajahi solusi otomasi terkini, dan dapatkan wawasan mendalam tentang masa depan manufaktur dan kawasan industri di Asia Tenggara.

Topik yang akan dibahas:
• Smart Factory & Industry 4.0
• Integrasi IoT dalam Manufaktur
• Praktik Industri Berkelanjutan
• Studi Kasus Transformasi Digital
• Keamanan dan Keselamatan Industri`,
		EventDate:            eventDate,
		Location:             "Jakarta Convention Center, Hall A, Jakarta Pusat",
		Price:                350000,
		PaymentBank:          "Bank Central Asia (BCA)",
		PaymentAccountNumber: "1234567890",
		PaymentAccountName:   "PT GatherHub Indonesia",
		AdminName:            "Budi Santoso",
		AdminWhatsapp:        "6281234567890",
	}

	return s.db.Create(&sample).Error
}
