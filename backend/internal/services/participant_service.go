package services

import (
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/trisfproject/gatherhub/internal/models"
	"gorm.io/gorm"
)

// ParticipantService handles participant registration business logic
type ParticipantService struct {
	db             *gorm.DB
	storageService *StorageService
}

// NewParticipantService creates a new ParticipantService
func NewParticipantService(db *gorm.DB, storageService *StorageService) *ParticipantService {
	return &ParticipantService{db: db, storageService: storageService}
}

// RegisterForm holds registration input data
type RegisterForm struct {
	FullName             string `form:"full_name"`
	Phone                string `form:"phone"`
	Email                string `form:"email"`
	City                 string `form:"city"`
	CompanyName          string `form:"company_name"`
	IndustrialEstate     string `form:"industrial_estate"`
	IndustrialEstateName string `form:"industrial_estate_name"`
	TelegramUsername     string `form:"telegram_username"`
	JobTitle             string `form:"job_title"` // optional
	EmergencyName        string `form:"emergency_name"`
	EmergencyRelationship string `form:"emergency_relationship"`
	EmergencyPhone       string `form:"emergency_phone"`
	OwnVehicle           string `form:"own_vehicle"`
	VehicleType          string `form:"vehicle_type"`
	LicensePlate         string `form:"license_plate"`
	CarpoolCanBring      string `form:"carpool_can_bring"`
	CarpoolSeats         string `form:"carpool_seats"`
	TShirtSize           string `form:"tshirt_size"`
	DepartureZone        string `form:"departure_zone"`
	DepartureZoneName    string `form:"departure_zone_name"`
}

// ParticipantStats holds dashboard counts by status
type ParticipantStats struct {
	Total          int64
	Pending        int64
	Verified       int64
	Rejected       int64
	CheckedIn      int64
	VerifiedTotal  int64
	AttendanceRate float64
}

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
var phoneRegex = regexp.MustCompile(`^(\+62|62|0)[0-9]{8,13}$`)

// Validate performs comprehensive validation on the form.
// Returns a slice of Indonesian-language error messages.
func (f *RegisterForm) Validate(event *models.Event) []string {
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

	// ── Conditional Validations based on Event Config ──
	if event.EnableIndustrialEstate {
		ie := strings.TrimSpace(f.IndustrialEstate)
		if ie == "" {
			errs = append(errs, "Kawasan Industri wajib dipilih")
		} else if ie == "Other" {
			if strings.TrimSpace(f.IndustrialEstateName) == "" {
				errs = append(errs, "Nama Kawasan Industri wajib diisi jika memilih Other")
			}
		}
	}

	if event.EnableTelegram {
		if strings.TrimSpace(f.TelegramUsername) == "" {
			errs = append(errs, "Username Telegram wajib diisi")
		}
	}

	if event.EnableJobTitle {
		if strings.TrimSpace(f.JobTitle) == "" {
			errs = append(errs, "Jabatan / Posisi wajib diisi")
		}
	}

	if event.EnableEmergencyContact {
		if strings.TrimSpace(f.EmergencyName) == "" {
			errs = append(errs, "Nama Kontak Darurat wajib diisi")
		}
		if strings.TrimSpace(f.EmergencyRelationship) == "" {
			errs = append(errs, "Hubungan Kontak Darurat wajib diisi")
		}
		if strings.TrimSpace(f.EmergencyPhone) == "" {
			errs = append(errs, "Nomor Telepon Kontak Darurat wajib diisi")
		} else if !phoneRegex.MatchString(strings.TrimSpace(f.EmergencyPhone)) {
			errs = append(errs, "Format Nomor Telepon Kontak Darurat tidak valid")
		}
	}

	if event.EnableVehicleInfo {
		if f.OwnVehicle == "true" {
			if strings.TrimSpace(f.VehicleType) == "" {
				errs = append(errs, "Jenis Kendaraan wajib dipilih")
			}
			if strings.TrimSpace(f.LicensePlate) == "" {
				errs = append(errs, "Nomor Polisi (Plat Nomor) wajib diisi")
			}
		}
	}

	if event.EnableCarpool {
		if f.CarpoolCanBring == "true" {
			seats, err := strconv.Atoi(f.CarpoolSeats)
			if err != nil || seats <= 0 {
				errs = append(errs, "Jumlah Kursi Tersedia wajib diisi dengan angka positif")
			}
		}
	}

	if event.EnableTShirtSize {
		ts := strings.TrimSpace(f.TShirtSize)
		if ts == "" {
			errs = append(errs, "Ukuran T-Shirt wajib dipilih")
		} else {
			validSizes := map[string]bool{"XS": true, "S": true, "M": true, "L": true, "XL": true, "XXL": true, "XXXL": true}
			if !validSizes[ts] {
				errs = append(errs, "Ukuran T-Shirt tidak valid")
			}
		}
	}

	if event.EnableTransportationCoordination {
		dz := strings.TrimSpace(f.DepartureZone)
		if dz == "" {
			errs = append(errs, "Zona Keberangkatan wajib dipilih")
		} else if dz == "Other" {
			if strings.TrimSpace(f.DepartureZoneName) == "" {
				errs = append(errs, "Nama Zona Keberangkatan wajib diisi jika memilih Other")
			}
		}

		if f.OwnVehicle != "true" && f.OwnVehicle != "false" {
			errs = append(errs, "Status kepemilikan kendaraan wajib dipilih")
		} else if f.OwnVehicle == "true" {
			vt := strings.TrimSpace(f.VehicleType)
			if vt != "Car" && vt != "Motorcycle" {
				errs = append(errs, "Jenis Kendaraan wajib dipilih (Mobil atau Motor)")
			}
			if strings.TrimSpace(f.LicensePlate) == "" {
				errs = append(errs, "Nomor Polisi (Plat Nomor) wajib diisi")
			}
			if f.CarpoolCanBring != "true" && f.CarpoolCanBring != "false" {
				errs = append(errs, "Pertanyaan tawaran tumpangan wajib dijawab")
			} else if f.CarpoolCanBring == "true" {
				seats, err := strconv.Atoi(f.CarpoolSeats)
				if err != nil || seats < 1 || seats > 4 {
					errs = append(errs, "Jumlah Kursi Tersedia wajib dipilih antara 1 sampai 4")
				}
			}
		}
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
	return s.storageService.SavePaymentProof(file)
}

func (s *ParticipantService) generateRegistrationNumber() (string, error) {
	prefix := "GH-" + time.Now().Format("2006") + "-"
	var count int64
	err := s.db.Model(&models.Participant{}).
		Where("registration_number LIKE ?", prefix+"%").
		Count(&count).Error
	if err != nil {
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
		EventID:               eventID,
		RegistrationNumber:    regNumber,
		FullName:              strings.TrimSpace(form.FullName),
		Phone:                 strings.TrimSpace(form.Phone),
		Email:                 strings.TrimSpace(strings.ToLower(form.Email)),
		City:                  strings.TrimSpace(form.City),
		CompanyName:           strings.TrimSpace(form.CompanyName),
		IndustrialEstate:      strings.TrimSpace(form.IndustrialEstate),
		IndustrialEstateName:  strings.TrimSpace(form.IndustrialEstateName),
		TelegramUsername:      strings.TrimSpace(form.TelegramUsername),
		EmergencyName:         strings.TrimSpace(form.EmergencyName),
		EmergencyRelationship: strings.TrimSpace(form.EmergencyRelationship),
		EmergencyPhone:        strings.TrimSpace(form.EmergencyPhone),
		OwnVehicle:            form.OwnVehicle == "true",
		VehicleType:           strings.TrimSpace(form.VehicleType),
		LicensePlate:          strings.TrimSpace(form.LicensePlate),
		CarpoolCanBring:       form.CarpoolCanBring == "true",
		TShirtSize:            strings.TrimSpace(form.TShirtSize),
		PaymentProof:          paymentFilename,
		Status:                models.StatusPending,
		DepartureZone:         strings.TrimSpace(form.DepartureZone),
		DepartureZoneName:     strings.TrimSpace(form.DepartureZoneName),
	}

	if strings.TrimSpace(form.JobTitle) != "" {
		jt := strings.TrimSpace(form.JobTitle)
		participant.JobTitle = &jt
	}

	if form.CarpoolCanBring == "true" {
		if seats, err := strconv.Atoi(form.CarpoolSeats); err == nil {
			participant.CarpoolSeats = seats
		}
	}

	if err := s.db.Create(participant).Error; err != nil {
		return nil, fmt.Errorf("gagal menyimpan data peserta: %w", err)
	}

	return participant, nil
}

// GetByID returns a participant by ID, preloading the associated Event
func (s *ParticipantService) GetByID(id uint) (*models.Participant, error) {
	var p models.Participant
	if err := s.db.Preload("Event").Preload("Driver").First(&p, id).Error; err != nil {
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
			"full_name ILIKE ? OR email ILIKE ? OR company_name ILIKE ? OR registration_number ILIKE ? OR phone ILIKE ?",
			like, like, like, like, like,
		)
	}

	if err := q.Find(&participants).Error; err != nil {
		return nil, err
	}
	return participants, nil
}

// GetPaginatedForAdmin returns a page of participants, the total count, and error
func (s *ParticipantService) GetPaginatedForAdmin(statusFilter, search string, page, limit int) ([]models.Participant, int64, error) {
	var participants []models.Participant
	var total int64

	q := s.db.Model(&models.Participant{})

	if statusFilter != "" {
		q = q.Where("status = ?", statusFilter)
	}

	if search != "" {
		like := "%" + search + "%"
		q = q.Where(
			"full_name ILIKE ? OR email ILIKE ? OR company_name ILIKE ? OR registration_number ILIKE ? OR phone ILIKE ?",
			like, like, like, like, like,
		)
	}

	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := q.Preload("Event").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&participants).Error

	return participants, total, err
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
		case models.StatusCheckedIn:
			stats.CheckedIn = r.Count
		}
	}

	stats.VerifiedTotal = stats.Verified + stats.CheckedIn
	if stats.VerifiedTotal > 0 {
		stats.AttendanceRate = float64(stats.CheckedIn) / float64(stats.VerifiedTotal) * 100.0
	}

	return stats, nil
}

// UpdateStatus changes a participant's status to VERIFIED or REJECTED
func (s *ParticipantService) UpdateStatus(id uint, status models.ParticipantStatus) (*models.Participant, error) {
	allowed := map[models.ParticipantStatus]bool{
		models.StatusVerified: true,
		models.StatusRejected: true,
	}
	if !allowed[status] {
		return nil, fmt.Errorf("status target tidak valid: %s", status)
	}

	var p models.Participant
	if err := s.db.Preload("Event").First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("participant not found")
		}
		return nil, err
	}

	// Enforce rules: PENDING -> VERIFIED, PENDING -> REJECTED
	if p.Status != models.StatusPending {
		return nil, fmt.Errorf("hanya peserta dengan status PENDING yang dapat diubah statusnya (status saat ini: %s)", p.Status)
	}

	p.Status = status
	now := time.Now()
	if status == models.StatusVerified {
		p.VerifiedAt = &now
		p.RejectedAt = nil
	} else if status == models.StatusRejected {
		p.RejectedAt = &now
		p.VerifiedAt = nil
	}

	if err := s.db.Save(&p).Error; err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	return &p, nil
}

// DayRegistration represents registration count for a specific day
type DayRegistration struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// CityDistribution represents participant count for a specific city
type CityDistribution struct {
	City  string `json:"city"`
	Count int64  `json:"count"`
}

// EstateDistribution represents participant count for a specific industrial estate
type EstateDistribution struct {
	IndustrialEstate string `json:"industrial_estate"`
	Count            int64  `json:"count"`
}

// DashboardAnalytics holds aggregated data for charts
type DashboardAnalytics struct {
	RegistrationsByDay   []DayRegistration    `json:"registrations_by_day"`
	ParticipantsByCity   []CityDistribution   `json:"participants_by_city"`
	ParticipantsByEstate []EstateDistribution `json:"participants_by_estate"`
}

// GetFilteredStats returns aggregated participant counts per status matching active filters
func (s *ParticipantService) GetFilteredStats(eventID uint, startDate, endDate string) (*ParticipantStats, error) {
	stats := &ParticipantStats{}

	q := s.db.Model(&models.Participant{})
	if eventID > 0 {
		q = q.Where("event_id = ?", eventID)
	}
	if startDate != "" {
		q = q.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("created_at <= ?", endDate+" 23:59:59")
	}

	type row struct {
		Status models.ParticipantStatus
		Count  int64
	}
	var rows []row

	if err := q.Select("status, count(*) as count").
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
		case models.StatusCheckedIn:
			stats.CheckedIn = r.Count
		}
	}

	stats.VerifiedTotal = stats.Verified + stats.CheckedIn
	if stats.VerifiedTotal > 0 {
		stats.AttendanceRate = float64(stats.CheckedIn) / float64(stats.VerifiedTotal) * 100.0
	}

	return stats, nil
}

// GetAttendanceStats returns participant stats filtered by event and date.
func (s *ParticipantService) GetAttendanceStats(eventID uint, date string) (*ParticipantStats, error) {
	stats := &ParticipantStats{}

	// Query standard status counts
	q := s.db.Model(&models.Participant{})
	if eventID > 0 {
		q = q.Where("event_id = ?", eventID)
	}
	if date != "" {
		q = q.Where("DATE(created_at) = ?", date)
	}

	type row struct {
		Status models.ParticipantStatus
		Count  int64
	}
	var rows []row

	if err := q.Select("status, count(*) as count").Group("status").Scan(&rows).Error; err != nil {
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
		case models.StatusCheckedIn:
			stats.CheckedIn = r.Count
		}
	}

	// For an attendance dashboard, if a date filter is applied, the checked-in counter
	// should represent the number of actual check-ins that occurred on that date.
	if date != "" {
		var checkedInCount int64
		qa := s.db.Model(&models.Attendance{}).Where("DATE(checked_in_at) = ?", date)
		if eventID > 0 {
			qa = qa.Where("event_id = ?", eventID)
		}
		if err := qa.Count(&checkedInCount).Error; err != nil {
			return nil, err
		}
		stats.CheckedIn = checkedInCount
	}

	stats.VerifiedTotal = stats.Verified + stats.CheckedIn
	if stats.VerifiedTotal > 0 {
		stats.AttendanceRate = float64(stats.CheckedIn) / float64(stats.VerifiedTotal) * 100.0
	}

	return stats, nil
}


// GetAnalytics returns aggregated chart data matching active filters
func (s *ParticipantService) GetAnalytics(eventID uint, startDate, endDate string) (*DashboardAnalytics, error) {
	analytics := &DashboardAnalytics{
		RegistrationsByDay:   []DayRegistration{},
		ParticipantsByCity:   []CityDistribution{},
		ParticipantsByEstate: []EstateDistribution{},
	}

	// 1. Registrations by day
	qDay := s.db.Model(&models.Participant{})
	if eventID > 0 {
		qDay = qDay.Where("event_id = ?", eventID)
	}
	if startDate != "" {
		qDay = qDay.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		qDay = qDay.Where("created_at <= ?", endDate+" 23:59:59")
	}

	if err := qDay.Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, count(*) as count").
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&analytics.RegistrationsByDay).Error; err != nil {
		return nil, err
	}

	// 2. Participants by City
	qCity := s.db.Model(&models.Participant{})
	if eventID > 0 {
		qCity = qCity.Where("event_id = ?", eventID)
	}
	if startDate != "" {
		qCity = qCity.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		qCity = qCity.Where("created_at <= ?", endDate+" 23:59:59")
	}

	if err := qCity.Select("city, count(*) as count").
		Where("city IS NOT NULL AND city != ''").
		Group("city").
		Order("count DESC").
		Limit(10).
		Scan(&analytics.ParticipantsByCity).Error; err != nil {
		return nil, err
	}

	// 3. Participants by Industrial Estate
	qEstate := s.db.Model(&models.Participant{})
	if eventID > 0 {
		qEstate = qEstate.Where("event_id = ?", eventID)
	}
	if startDate != "" {
		qEstate = qEstate.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		qEstate = qEstate.Where("created_at <= ?", endDate+" 23:59:59")
	}

	if err := qEstate.Select("industrial_estate, count(*) as count").
		Where("industrial_estate IS NOT NULL AND industrial_estate != ''").
		Group("industrial_estate").
		Order("count DESC").
		Limit(10).
		Scan(&analytics.ParticipantsByEstate).Error; err != nil {
		return nil, err
	}

	return analytics, nil
}

// GetLatestRegistrations returns the most recent registrants
func (s *ParticipantService) GetLatestRegistrations(eventID uint, limit int) ([]models.Participant, error) {
	var participants []models.Participant
	q := s.db.Model(&models.Participant{})
	if eventID > 0 {
		q = q.Where("event_id = ?", eventID)
	}
	err := q.Preload("Event").Order("created_at DESC").Limit(limit).Find(&participants).Error
	return participants, err
}

// GetLatestVerifications returns the most recent verified participants
func (s *ParticipantService) GetLatestVerifications(eventID uint, limit int) ([]models.Participant, error) {
	var participants []models.Participant
	q := s.db.Model(&models.Participant{}).Where("status = ? AND verified_at IS NOT NULL", models.StatusVerified)
	if eventID > 0 {
		q = q.Where("event_id = ?", eventID)
	}
	err := q.Preload("Event").Order("verified_at DESC").Limit(limit).Find(&participants).Error
	return participants, err
}

// GetDB returns the database client instance
func (s *ParticipantService) GetDB() *gorm.DB {
	return s.db
}
