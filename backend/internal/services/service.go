package services

import (
	"gorm.io/gorm"
)

// BaseService provides common database access
type BaseService struct {
	DB *gorm.DB
}

// NewBaseService creates a new BaseService
func NewBaseService(db *gorm.DB) *BaseService {
	return &BaseService{DB: db}
}
