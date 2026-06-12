package models

import (
	"time"
)

// Admin represents an administrator account
type Admin struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string    `json:"name" gorm:"size:255"`
	Username     string    `json:"username" gorm:"uniqueIndex;not null;size:255"`
	Email        string    `json:"email" gorm:"uniqueIndex;not null;size:255"`
	PasswordHash string    `json:"-" gorm:"not null;size:255"`
	Role         string    `json:"role" gorm:"not null;default:'ADMIN';size:20"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
