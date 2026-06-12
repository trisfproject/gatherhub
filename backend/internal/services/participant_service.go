package services

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
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

// ParticipantStats holds dashboard counts by status
type ParticipantStats struct {
	Total    int64
	Pending  int64
	Verified int64
	Rejected int64
}

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
var phoneRegex = regexp.MustCompile(`^(\+62|62|0)[0-9]{8,13}$`)

// Validate performs comprehensive validation on the form.
// Returns a slice of Indonesian-language error messages.
func (f *RegisterForm) Validate() []string {
	var errs []string

	if strings.TrimSpace(f.FullName) == "" {
		errs = append(errs, "Nama Lengkap wajib diisi")
	} else if len(strings.TrimSpace(f.FullName)) < 3 {
		errs = append(errs, "Nama Lengkap minimal 3 karakter")
	}

	phone := strings.TrimSpace(f.Phone)
	if phone == "" {
		errs = append(errs, "Nomor WhatsApp wajib diisi")
	} else if !phoneRegex.MatchString(phone) {
		errs = append(errs, "Format Nomor WhatsApp tidak valid (contoh: 081234567890)")
	}

	email := strings.TrimSpace(f.Email)
	if email == "" {
		errs = append(errs, "Email wajib diisi")
	} else if !emailRegex.MatchString(email) {
		errs = append(errs, "Format email tidak valid")
	}

	if strings.TrimSpace(f.City) == "" {
		errs = append(errs, "Kota wajib diisi")
	}
	if strings.TrimSpace(f.CompanyName) == "" {
		errs = append(errs, "Nama Perusahaan wajib diisi")
	}
	if strings.TrimSpace(f.IndustrialEstate) == "" {
		errs = append(errs, "Kawasan Industri wajib diisi")
	}
	if strings.TrimSpace(f.TelegramUsername) == "" {
		errs = append(errs, "Username Telegram wajib diisi")
	}

	return errs
}

// ValidateFile validates the uploaded payment proof file
func ValidateFile(file *multipart.FileHeader) error {
	if file == nil {
		return fmt.Errorf("Bukti pembayaran wajib diunggah")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".pdf": true}
	if !allowed[ext] {
		return fmt.Errorf("Tipe file tidak diizinkan. Gunakan JPG, JPEG, PNG, atau PDF")
	}

	if file.Size > 10*1024*1024 {
		return fmt.Errorf("Ukuran file melebihi batas maksimum 10MB")
	}
	if file.Size == 0 {
		return fmt.Errorf("File bukti pembayaran tidak boleh kosong")
	}

	return nil
}

// SavePaymentProof saves the uploaded file to storage/payments/ and returns the filename
func (s *ParticipantService) SavePaymentProof(file *multipart.FileHeader) (string, error) {
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return "", fmt.Errorf("gagal membuat direktori upload: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := fmt.Sprintf("payment_%d%s", time.Now().UnixNano(), ext)
	fullPath := filepath.Join(s.uploadDir, filename)

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("gagal membuka file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("gagal menyimpan file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("gagal menulis file: %w", err)
	}

	return filename, nil
}

// generateRegistrationNumber creates a unique GH-YYYY-NNNN registration number
func (s *ParticipantService) generateRegistrationNumber() (string, error) {
	year := time.Now().Year()
	prefix := fmt.Sprintf("GH-%d-", year)

	var count int64
	if err := s.db.Model(&models.Participant{}).
		Where("registration_number LIKE ?", prefix+"%").
		Count(&count).Error; err != nil {
		return "", fmt.Errorf("gagal menghitung nomor registrasi: %w", err)
	}

	return fmt.Sprintf("%s%04d", prefix, count+1), nil
}

// Create registers a new participant for an event
func (s *ParticipantService) Create(eventID uint, form *RegisterForm, paymentFilename string) (*models.Participant, error) {
	regNumber, err := s.generateRegistrationNumber()
	if err != nil {
		return nil, err
	}

	participant := &models.Participant{
		EventID:            eventID,
		RegistrationNumber: regNumber,
		FullName:           strings.TrimSpace(form.FullName),
		Phone:              strings.TrimSpace(form.Phone),
		Email:              strings.TrimSpace(strings.ToLower(form.Email)),
		City:               strings.TrimSpace(form.City),
		CompanyName:        strings.TrimSpace(form.CompanyName),
		IndustrialEstate:   strings.TrimSpace(form.IndustrialEstate),
		TelegramUsername:   strings.TrimSpace(form.TelegramUsername),
		PaymentProof:       paymentFilename,
		Status:             models.StatusPending,
	}

	if jt := strings.TrimSpace(form.JobTitle); jt != "" {
		participant.JobTitle = &jt
	}

	if err := s.db.Create(participant).Error; err != nil {
		return nil, fmt.Errorf("gagal menyimpan data peserta: %w", err)
	}

	return participant, nil
}

// GetByID returns a participant by ID, preloading the associated Event
func (s *ParticipantService) GetByID(id uint) (*models.Participant, error) {
	var p models.Participant
	if err := s.db.Preload("Event").First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ─────────────────────── Admin Methods ───────────────────────

// GetAllForAdmin returns all participants ordered by created_at desc with their events preloaded.
// Optional status filter: "", "PENDING", "VERIFIED", "REJECTED"
// Optional search: partial match on name, email, company, or registration number
func (s *ParticipantService) GetAllForAdmin(statusFilter, search string) ([]models.Participant, error) {
	var participants []models.Participant

	q := s.db.Preload("Event").Order("created_at DESC")

	if statusFilter != "" {
		q = q.Where("status = ?", statusFilter)
	}

	if search != "" {
		like := "%" + search + "%"
		q = q.Where(
			"full_name ILIKE ? OR email ILIKE ? OR company_name ILIKE ? OR registration_number ILIKE ?",
			like, like, like, like,
		)
	}

	if err := q.Find(&participants).Error; err != nil {
		return nil, err
	}
	return participants, nil
}

// GetStats returns aggregate participant counts per status
func (s *ParticipantService) GetStats() (*ParticipantStats, error) {
	stats := &ParticipantStats{}

	type row struct {
		Status models.ParticipantStatus
		Count  int64
	}
	var rows []row

	if err := s.db.Model(&models.Participant{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, r := range rows {
		stats.Total += r.Count
		switch r.Status {
		case models.StatusPending:
			stats.Pending = r.Count
		case models.StatusVerified:
			stats.Verified = r.Count
		case models.StatusRejected:
			stats.Rejected = r.Count
		}
	}

	return stats, nil
}

// UpdateStatus changes a participant's status to VERIFIED or REJECTED
func (s *ParticipantService) UpdateStatus(id uint, status models.ParticipantStatus) (*models.Participant, error) {
	allowed := map[models.ParticipantStatus]bool{
		models.StatusVerified: true,
		models.StatusRejected: true,
		models.StatusPending:  true,
	}
	if !allowed[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	var p models.Participant
	if err := s.db.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("participant not found")
		}
		return nil, err
	}

	p.Status = status
	if err := s.db.Save(&p).Error; err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	return &p, nil
}
