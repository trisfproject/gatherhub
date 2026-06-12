package models

import (
	"time"
)

// AuditLog represents a logged administrative or system action in GatherHub.
type AuditLog struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Actor        string    `json:"actor" gorm:"size:255;not null"`
	Action       string    `json:"action" gorm:"size:100;not null;index"`        // CREATE, UPDATE, DELETE, PUBLISH, CLOSE, VERIFY, REJECT, SENT, FAILED
	ResourceType string    `json:"resource_type" gorm:"size:100;not null;index"` // EVENT, PARTICIPANT, NOTIFICATION
	ResourceID   uint      `json:"resource_id" gorm:"index"`
	OldValue     *string   `json:"old_value" gorm:"type:jsonb"`
	NewValue     *string   `json:"new_value" gorm:"type:jsonb"`
	IPAddress    string    `json:"ip_address" gorm:"size:45"`
	UserAgent    string    `json:"user_agent" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at"`
}
