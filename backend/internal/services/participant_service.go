package services

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gatherhub/backend/internal/models"
	"gorm.io/gorm"
)

// ParticipantService handles participant registration business logic
type ParticipantService struct {
	db        *gorm.DB
	uploadDir string
}

// NewParticipantService creates a new ParticipantService
func NewParticipantService(db *gorm.DB, uploadDir string) *ParticipantService {
	return &ParticipantService{db: db, uploadDir: uploadDir}
}

// RegisterForm holds registration input data
type RegisterForm struct {
	FullName         string `form:"full_name"`
	Phone            string `form:"phone"`
	Email            string `form:"email"`
	City             string `form:"city"`
	CompanyName      string `form:"company_name"`
	IndustrialEstate string `form:"industrial_estate"`
	TelegramUsername string `form:"telegram_username"`
	JobTitle         string `form:"job_title"` // optional
}

// Validate performs basic validation on the form
func (f *RegisterForm) Validate() []string {
	var errs []string
	if strings.TrimSpace(f.FullName) == "" {
		errs = append(errs, "Full Name is required")
	}
	if strings.TrimSpace(f.Phone) == "" {
		errs = append(errs, "WhatsApp number is required")
	}
	if strings.TrimSpace(f.Email) == "" {
		errs = append(errs, "Email is required")
	} else if !strings.Contains(f.Email, "@") {
		errs = append(errs, "Email format is invalid")
	}
	if strings.TrimSpace(f.City) == "" {
		errs = append(errs, "City is required")
	}
	if strings.TrimSpace(f.CompanyName) == "" {
		errs = append(errs, "Company Name is required")
	}
	if strings.TrimSpace(f.IndustrialEstate) == "" {
		errs = append(errs, "Industrial Estate is required")
	}
	if strings.TrimSpace(f.TelegramUsername) == "" {
		errs = append(errs, "Telegram Username is required")
	}
	return errs
}

// SavePaymentProof saves the uploaded file and returns the relative file path
func (s *ParticipantService) SavePaymentProof(file *multipart.FileHeader) (string, error) {
	// Ensure upload directory exists
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".pdf": true, ".webp": true}
	if !allowedExts[ext] {
		return "", fmt.Errorf("file type %s is not allowed. Use JPG, PNG, PDF, or WEBP", ext)
	}

	// Validate file size (max 5MB)
	if file.Size > 5*1024*1024 {
		return "", fmt.Errorf("file size exceeds 5MB limit")
	}

	// Generate unique filename
	filename := fmt.Sprintf("payment_%d%s", time.Now().UnixNano(), ext)
	fullPath := filepath.Join(s.uploadDir, filename)

	// Open source file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Write to destination
	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return "", fmt.Errorf("failed to write file: %w", writeErr)
			}
		}
		if readErr != nil {
			break
		}
	}

	return filename, nil
}

// Create registers a new participant for an event
func (s *ParticipantService) Create(eventID uint, form *RegisterForm, paymentFilename string) (*models.Participant, error) {
	participant := &models.Participant{
		EventID:          eventID,
		FullName:         strings.TrimSpace(form.FullName),
		Phone:            strings.TrimSpace(form.Phone),
		Email:            strings.TrimSpace(strings.ToLower(form.Email)),
		City:             strings.TrimSpace(form.City),
		CompanyName:      strings.TrimSpace(form.CompanyName),
		IndustrialEstate: strings.TrimSpace(form.IndustrialEstate),
		TelegramUsername: strings.TrimSpace(form.TelegramUsername),
		PaymentProof:     paymentFilename,
		Status:           models.StatusPending,
	}

	// Set optional job title
	if strings.TrimSpace(form.JobTitle) != "" {
		jt := strings.TrimSpace(form.JobTitle)
		participant.JobTitle = &jt
	}

	if err := s.db.Create(participant).Error; err != nil {
		return nil, fmt.Errorf("failed to save participant: %w", err)
	}

	return participant, nil
}

// GetByID returns a participant by ID
func (s *ParticipantService) GetByID(id uint) (*models.Participant, error) {
	var p models.Participant
	if err := s.db.Preload("Event").First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}
