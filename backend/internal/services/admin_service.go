package services

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/trisfproject/gatherhub/internal/models"
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
	var count int64
	if err := s.db.Model(&models.Admin{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count admin accounts: %w", err)
	}

	if count == 0 {
		log.Println("No administrator accounts found.")
		username := os.Getenv("INITIAL_ADMIN_USERNAME")
		password := os.Getenv("INITIAL_ADMIN_PASSWORD")
		email := os.Getenv("INITIAL_ADMIN_EMAIL")

		if username == "" || password == "" || email == "" {
			return nil
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash default admin password: %w", err)
		}

		defaultAdmin := models.Admin{
			Name:         username,
			Username:     username,
			Email:        email,
			PasswordHash: string(hash),
			Role:         "SUPER_ADMIN",
		}

		if err := s.db.Create(&defaultAdmin).Error; err != nil {
			return fmt.Errorf("failed to create default admin: %w", err)
		}

		log.Println("Default admin created")
	}

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
