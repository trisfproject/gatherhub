package database

import (
	"fmt"
	"log"

	"github.com/trisfproject/gatherhub/internal/config"
	"github.com/trisfproject/gatherhub/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect establishes a PostgreSQL connection using GORM
func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	log.Println("Database connection established successfully")
	return db, nil
}

// AutoMigrate runs GORM auto migration for all models
func AutoMigrate(db *gorm.DB) error {
	log.Println("Running database migrations...")
	err := db.AutoMigrate(
		&models.Event{},
		&models.Participant{},
		&models.Admin{},
		&models.NotificationLog{},
	)
	if err != nil {
		return fmt.Errorf("auto migration failed: %w", err)
	}
	log.Println("Database migrations completed successfully")
	return nil
}
