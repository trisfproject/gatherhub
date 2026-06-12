package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	AppPort string
	AppEnv  string

	// Database
	DBHost    string
	DBPort    string
	DBUser    string
	DBPass    string
	DBName    string
	DBSSLMode string

	// Storage — single root path; sub-directories are derived by StorageConfig.
	// Set STORAGE_PATH to an absolute path outside the repository.
	// Example (local): STORAGE_PATH=/home/langit/Dev/event/gatherhub-storage
	// Example (Docker): STORAGE_PATH=/storage
	StoragePath string

	// Frontend (served by Fiber for local dev)
	FrontendDir string

	// Admin credentials (set via env, never hardcoded in production)
	AdminUsername string
	AdminPassword string
	SessionSecret string
}

// Load reads configuration from environment variables (and .env file if present).
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	return &Config{
		AppPort: getEnv("APP_PORT", "3000"),
		AppEnv:  getEnv("APP_ENV", "development"),

		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "5432"),
		DBUser:    getEnv("DB_USER", "gatherhub"),
		DBPass:    getEnv("DB_PASSWORD", "gatherhub"),
		DBName:    getEnv("DB_NAME", "gatherhub"),
		DBSSLMode: getEnv("DB_SSLMODE", "disable"),

		// Default points to a sibling directory of the repo root (outside git).
		// When running inside Docker, override with STORAGE_PATH=/storage.
		StoragePath: getEnv("STORAGE_PATH", "../gatherhub-storage"),

		FrontendDir: getEnv("FRONTEND_DIR", "../frontend"),

		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123"),
		SessionSecret: getEnv("SESSION_SECRET", "gatherhub-secret-change-in-production"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
