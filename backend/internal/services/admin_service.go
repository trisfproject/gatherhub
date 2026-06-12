package services

import (
	"errors"
	"fmt"
	"log"

	"github.com/gatherhub/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminService handles administrator account operations
type AdminService struct {
	db *gorm.DB
}

// NewAdminService creates a new AdminService instance
func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{db: db}
}

// SeedDefaultAdmin creates the default admin user if it does not already exist
func (s *AdminService) SeedDefaultAdmin() error {
	var admin models.Admin
	err := s.db.Where("username = ?", "trisf").First(&admin).Error
	if err == nil {
		if admin.Role != "SUPER_ADMIN" {
			admin.Role = "SUPER_ADMIN"
			s.db.Save(&admin)
			log.Println("Default admin role updated to SUPER_ADMIN")
		}
		log.Println("Default admin already exists")
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to query admin table: %w", err)
	}

	// Password hash using bcrypt
	passwordBytes := []byte("samudera")
	hash, err := bcrypt.GenerateFromPassword(passwordBytes, bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash default admin password: %w", err)
	}

	defaultAdmin := models.Admin{
		Name:         "trisf",
		Username:     "trisf",
		Email:        "admin@gatherhub.local",
		PasswordHash: string(hash),
		Role:         "SUPER_ADMIN",
	}

	if err := s.db.Create(&defaultAdmin).Error; err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	log.Println("Default admin created")
	return nil
}

// Authenticate verifies the username and password against the database
func (s *AdminService) Authenticate(username, password string) (*models.Admin, error) {
	var admin models.Admin
	err := s.db.Where("username = ?", username).First(&admin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("username atau password salah")
		}
		return nil, fmt.Errorf("error querying database: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("username atau password salah")
	}

	return &admin, nil
}

// GetAllAdmins returns all registered admins ordered by username
func (s *AdminService) GetAllAdmins() ([]models.Admin, error) {
	var admins []models.Admin
	err := s.db.Order("username ASC").Find(&admins).Error
	return admins, err
}

// GetAdminByID retrieves a single admin by ID
func (s *AdminService) GetAdminByID(id uint) (*models.Admin, error) {
	var admin models.Admin
	err := s.db.First(&admin, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("admin with id %d not found", id)
	}
	return &admin, err
}

// GetByUsername retrieves a single admin by username
func (s *AdminService) GetByUsername(username string) (*models.Admin, error) {
	var admin models.Admin
	err := s.db.Where("username = ?", username).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("admin with username %s not found", username)
	}
	return &admin, err
}

// GetByEmail retrieves a single admin by email
func (s *AdminService) GetByEmail(email string) (*models.Admin, error) {
	var admin models.Admin
	err := s.db.Where("email = ?", email).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("admin with email %s not found", email)
	}
	return &admin, err
}

// CreateAdmin registers a new administrator account
func (s *AdminService) CreateAdmin(admin *models.Admin) error {
	return s.db.Create(admin).Error
}

// UpdateAdmin saves changes to an existing administrator account
func (s *AdminService) UpdateAdmin(admin *models.Admin) error {
	return s.db.Save(admin).Error
}

// DeleteAdmin removes an administrator account by ID
func (s *AdminService) DeleteAdmin(id uint) error {
	return s.db.Delete(&models.Admin{}, id).Error
}
