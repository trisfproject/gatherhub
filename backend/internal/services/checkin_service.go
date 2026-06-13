package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/trisfproject/gatherhub/internal/models"
	"gorm.io/gorm"
)

// QRPayload represents the data stored in a participant's QR code.
type QRPayload struct {
	ParticipantID      uint   `json:"participant_id"`
	RegistrationNumber string `json:"registration_number"`
	EventID            uint   `json:"event_id"`
}

// CheckinService handles event check-in rules and QR code operations.
type CheckinService struct {
	db            *gorm.DB
	signingSecret string
}

// NewCheckinService creates a new CheckinService.
func NewCheckinService(db *gorm.DB, signingSecret string) *CheckinService {
	return &CheckinService{
		db:            db,
		signingSecret: signingSecret,
	}
}

// GenerateQRToken generates a signed QR code payload string.
func (s *CheckinService) GenerateQRToken(participantID, eventID uint, regNum string) (string, error) {
	payload := QRPayload{
		ParticipantID:      participantID,
		RegistrationNumber: regNum,
		EventID:            eventID,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal QR payload: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(bytes)

	h := hmac.New(sha256.New, []byte(s.signingSecret))
	h.Write([]byte(encoded))
	signature := hex.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%s.%s", encoded, signature), nil
}

// VerifyQRToken decodes and verifies the signature of a QR code token.
func (s *CheckinService) VerifyQRToken(token string) (*QRPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("format token tidak valid")
	}

	encoded := parts[0]
	signature := parts[1]

	h := hmac.New(sha256.New, []byte(s.signingSecret))
	h.Write([]byte(encoded))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, errors.New("tanda tangan token tidak valid")
	}

	bytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("gagal memecahkan kode payload: %w", err)
	}

	var payload QRPayload
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return nil, fmt.Errorf("gagal membaca data payload: %w", err)
	}

	return &payload, nil
}

// Checkin processes a check-in request for a participant.
func (s *CheckinService) Checkin(participantID, eventID uint, checkedInBy string) (*models.Attendance, error) {
	var att models.Attendance

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Get participant
		var p models.Participant
		if err := tx.Preload("Event").First(&p, participantID).Error; err != nil {
			return errors.New("peserta tidak ditemukan")
		}

		// 2. Validate status and prevent duplicate check-in
		if p.Status == models.StatusCheckedIn {
			return errors.New("Participant already checked in.")
		}

		var count int64
		tx.Model(&models.Attendance{}).Where("participant_id = ? AND event_id = ?", participantID, eventID).Count(&count)
		if count > 0 {
			return errors.New("Participant already checked in.")
		}

		if p.Status != models.StatusVerified {
			return errors.New("hanya peserta yang telah VERIFIED yang diperbolehkan check-in")
		}

		// 3. Update participant status to CHECKED_IN
		p.Status = models.StatusCheckedIn
		if err := tx.Save(&p).Error; err != nil {
			return fmt.Errorf("failed to update participant status: %w", err)
		}

		// 4. Create attendance record
		att = models.Attendance{
			ParticipantID: participantID,
			EventID:       eventID,
			CheckedInAt:   time.Now(),
			CheckedInBy:   checkedInBy,
			CreatedAt:     time.Now(),
		}

		if err := tx.Create(&att).Error; err != nil {
			return fmt.Errorf("gagal menyimpan data check-in: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Fetch created attendance with associations
	if err := s.db.Preload("Participant").Preload("Event").First(&att, att.ID).Error; err != nil {
		return &att, nil // fallback to returning the object without preloaded fields if preload fails
	}

	return &att, nil
}

// IsCheckedIn checks if a participant has already checked in.
func (s *CheckinService) IsCheckedIn(participantID uint) (bool, *models.Attendance, error) {
	var att models.Attendance
	err := s.db.Preload("Participant").Preload("Event").Where("participant_id = ?", participantID).First(&att).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, &att, nil
}

// GetAttendance retrieves an attendance record by participant and event IDs, preloading associated entities.
func (s *CheckinService) GetAttendance(participantID, eventID uint) (*models.Attendance, error) {
	var att models.Attendance
	err := s.db.Preload("Participant").Preload("Event").
		Where("participant_id = ? AND event_id = ?", participantID, eventID).
		First(&att).Error
	if err != nil {
		return nil, err
	}
	return &att, nil
}

// GetLatestCheckins returns a list of the most recent check-ins, preloading participant and event relations.
func (s *CheckinService) GetLatestCheckins(eventID uint, date string, search string, limit int) ([]models.Attendance, error) {
	var attendances []models.Attendance
	q := s.db.Preload("Participant").Preload("Event")

	if eventID > 0 {
		q = q.Where("attendances.event_id = ?", eventID)
	}

	if date != "" {
		q = q.Where("DATE(attendances.checked_in_at) = ?", date)
	}

	if search != "" {
		q = q.Joins("JOIN participants ON participants.id = attendances.participant_id").
			Select("attendances.*").
			Where("participants.full_name ILIKE ? OR participants.registration_number ILIKE ? OR participants.company_name ILIKE ?",
				"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if limit <= 0 {
		limit = 50
	}

	err := q.Order("attendances.checked_in_at DESC").Limit(limit).Find(&attendances).Error
	if err != nil {
		return nil, err
	}

	return attendances, nil
}


