package services

import (
	"errors"
	"fmt"

	"github.com/trisfproject/gatherhub/internal/models"
	"gorm.io/gorm"
)

// SponsorService handles sponsor business logic
type SponsorService struct {
	db *gorm.DB
}

// NewSponsorService creates a new SponsorService
func NewSponsorService(db *gorm.DB) *SponsorService {
	return &SponsorService{db: db}
}

// GetAllForAdmin returns all sponsors ordered by event_id desc, display_order asc, name asc, preloading the event
func (s *SponsorService) GetAllForAdmin() ([]models.Sponsor, error) {
	var sponsors []models.Sponsor
	result := s.db.Preload("Event").Order("event_id DESC, display_order ASC, name ASC").Find(&sponsors)
	return sponsors, result.Error
}

// GetByID returns a single sponsor by ID
func (s *SponsorService) GetByID(id uint) (*models.Sponsor, error) {
	var sponsor models.Sponsor
	result := s.db.Preload("Event").First(&sponsor, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("sponsor with id %d not found", id)
	}
	return &sponsor, result.Error
}

// Create saves a new sponsor
func (s *SponsorService) Create(sponsor *models.Sponsor) error {
	return s.db.Create(sponsor).Error
}

// Update updates an existing sponsor
func (s *SponsorService) Update(sponsor *models.Sponsor) error {
	return s.db.Save(sponsor).Error
}

// Delete removes a sponsor
func (s *SponsorService) Delete(id uint) error {
	return s.db.Delete(&models.Sponsor{}, id).Error
}

// GetActiveForEvent returns active sponsors for an event ordered by display_order asc, name asc
func (s *SponsorService) GetActiveForEvent(eventID uint) ([]models.Sponsor, error) {
	var sponsors []models.Sponsor
	result := s.db.Where("event_id = ? AND active = ?", eventID, true).Order("display_order ASC, name ASC").Find(&sponsors)
	return sponsors, result.Error
}

// GetCountByEvent returns the number of sponsors for an event
func (s *SponsorService) GetCountByEvent(eventID uint) (int64, error) {
	var count int64
	result := s.db.Model(&models.Sponsor{}).Where("event_id = ?", eventID).Count(&count)
	return count, result.Error
}
