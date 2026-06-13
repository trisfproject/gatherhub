package models

import (
	"time"
)

// Task Categories
const (
	TaskCategoryRegistration   = "Registration"
	TaskCategoryTransportation = "Transportation"
	TaskCategoryMerchandise    = "Merchandise"
	TaskCategorySponsor        = "Sponsor"
	TaskCategoryVenue          = "Venue"
	TaskCategoryBroadcast      = "Broadcast"
	TaskCategoryLogistics      = "Logistics"
	TaskCategoryOther          = "Other"
)

// Task Priorities
const (
	TaskPriorityLow      = "Low"
	TaskPriorityMedium   = "Medium"
	TaskPriorityHigh     = "High"
	TaskPriorityCritical = "Critical"
)

// Task Statuses
const (
	TaskStatusTodo       = "Todo"
	TaskStatusInProgress = "In Progress"
	TaskStatusDone       = "Done"
	TaskStatusCancelled  = "Cancelled"
)

// Task represents a task needed to prepare an event.
type Task struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	EventID     uint      `json:"event_id" gorm:"not null"`
	Title       string    `json:"title" gorm:"not null;size:255"`
	Description string    `json:"description" gorm:"type:text"`
	Category    string    `json:"category" gorm:"not null;size:50"`
	Priority    string    `json:"priority" gorm:"not null;size:50"`
	DueDate     time.Time `json:"due_date" gorm:"not null"`
	AssignedTo  string    `json:"assigned_to" gorm:"size:255"`
	Status      string    `json:"status" gorm:"not null;default:'Todo';size:50"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Associations
	Event Event `json:"event,omitempty" gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE"`
}
