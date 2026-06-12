package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/trisfproject/gatherhub/internal/models"
	"gorm.io/gorm"
)

// AuditLogService handles logging and retrieval of administrative / system actions
type AuditLogService struct {
	db *gorm.DB
}

// NewAuditLogService creates a new AuditLogService
func NewAuditLogService(db *gorm.DB) *AuditLogService {
	return &AuditLogService{db: db}
}

// Log writes a new log entry to the audit_logs table.
func (s *AuditLogService) Log(actor, action, resourceType string, resourceID uint, oldValue, newValue interface{}, ip, userAgent string) error {
	var oldValPtr *string
	if oldValue != nil {
		str := toJSONString(oldValue)
		if str != "" {
			oldValPtr = &str
		}
	}

	var newValPtr *string
	if newValue != nil {
		str := toJSONString(newValue)
		if str != "" {
			newValPtr = &str
		}
	}

	logEntry := models.AuditLog{
		Actor:        actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		OldValue:     oldValPtr,
		NewValue:     newValPtr,
		IPAddress:    ip,
		UserAgent:    userAgent,
		CreatedAt:    time.Now(),
	}

	return s.db.Create(&logEntry).Error
}

// QueryLogs retrieves audit logs based on search terms and filters.
func (s *AuditLogService) QueryLogs(search, resourceFilter, actionFilter, startDate, endDate string) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	query := s.db.Order("created_at DESC")

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where(
			"(actor ILIKE ? OR action ILIKE ? OR resource_type ILIKE ? OR old_value ILIKE ? OR new_value ILIKE ?)",
			searchTerm, searchTerm, searchTerm, searchTerm, searchTerm,
		)
	}

	if resourceFilter != "" {
		query = query.Where("resource_type = ?", resourceFilter)
	}

	if actionFilter != "" {
		query = query.Where("action = ?", actionFilter)
	}

	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			// Start of the day
			query = query.Where("created_at >= ?", t)
		}
	}

	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			// End of the day (23:59:59)
			endOfDay := t.Add(24*time.Hour - time.Second)
			query = query.Where("created_at <= ?", endOfDay)
		}
	}

	err := query.Find(&logs).Error
	return logs, err
}

func toJSONString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return ""
		}
		// Check if it's already valid JSON. If not, serialize as JSON string.
		var js json.RawMessage
		if json.Unmarshal([]byte(val), &js) == nil {
			return val
		}
		bytes, _ := json.Marshal(val)
		return string(bytes)
	case []byte:
		if len(val) == 0 {
			return ""
		}
		var js json.RawMessage
		if json.Unmarshal(val, &js) == nil {
			return string(val)
		}
		bytes, _ := json.Marshal(string(val))
		return string(bytes)
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf(`{"error": %q}`, err.Error())
		}
		return string(bytes)
	}
}
