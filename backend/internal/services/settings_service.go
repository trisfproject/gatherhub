package services

import (
	"strconv"
	"sync"
	"time"

	"github.com/trisfproject/gatherhub/internal/models"
	"gorm.io/gorm"
)

var defaultSettings = map[string]struct {
	Value    string
	Category string
}{
	"site_name":            {"GatherHub", "General"},
	"site_description":     {"Platform Pendaftaran & Check-in Acara", "General"},
	"footer_text":          {"© 2026 GatherHub. All rights reserved.", "General"},
	"registration_enabled": {"true", "Registration"},
	"maintenance_mode":     {"false", "Registration"},
	"support_name":         {"GatherHub Support", "Contact"},
	"support_whatsapp":     {"6281234567890", "Contact"},
	"support_email":        {"support@gatherhub.local", "Contact"},
	"whatsapp_enabled":     {"true", "Notification"},
	"notification_enabled": {"true", "Notification"},
	"storage_path":         {"/storage", "Storage"},
}

// SettingsService handles thread-safe caching and GORM operations for system configuration settings.
type SettingsService struct {
	db    *gorm.DB
	cache map[string]string
	mu    sync.RWMutex
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{
		db:    db,
		cache: make(map[string]string),
	}
}

// Load seeds default settings and fetches all configurations from the database into the in-memory cache.
func (s *SettingsService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Seed defaults if missing
	for key, def := range defaultSettings {
		var setting models.Setting
		err := s.db.Where("key = ?", key).First(&setting).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				newSetting := models.Setting{
					Key:      key,
					Value:    def.Value,
					Category: def.Category,
				}
				if err := s.db.Create(&newSetting).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	// Fetch all settings
	var settings []models.Setting
	if err := s.db.Find(&settings).Error; err != nil {
		return err
	}

	// Reset cache
	s.cache = make(map[string]string)
	for _, setting := range settings {
		s.cache[setting.Key] = setting.Value
	}

	return nil
}

// Get retrieves a setting's value from the memory cache, falling back to compile-time defaults.
func (s *SettingsService) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if val, ok := s.cache[key]; ok {
		return val
	}
	if def, ok := defaultSettings[key]; ok {
		return def.Value
	}
	return ""
}

// GetBool retrieves a setting's value parsed as a boolean.
func (s *SettingsService) GetBool(key string) bool {
	val := s.Get(key)
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return parsed
}

// UpdateMany updates multiple settings in the database and reloads the memory cache.
func (s *SettingsService) UpdateMany(settings map[string]string) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for key, value := range settings {
			var setting models.Setting
			err := tx.Where("key = ?", key).First(&setting).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					// Fallback category if key is in defaults
					cat := "General"
					if def, ok := defaultSettings[key]; ok {
						cat = def.Category
					}
					newSetting := models.Setting{
						Key:      key,
						Value:    value,
						Category: cat,
					}
					if err := tx.Create(&newSetting).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			} else {
				setting.Value = value
				setting.UpdatedAt = time.Now()
				if err := tx.Save(&setting).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Reload the settings memory cache
	return s.Load()
}
