package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	AppPort   string
	AppEnv    string
	DBHost    string
	DBPort    string
	DBUser    string
	DBPass    string
	DBName    string
	DBSSLMode string
	UploadDir string
}

// Load reads configuration from environment variables (and .env file if present)
func Load() *Config {
	// Attempt to load .env file (non-fatal if missing)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	return &Config{
		AppPort:   getEnv("APP_PORT", "3000"),
		AppEnv:    getEnv("APP_ENV", "development"),
		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "5432"),
		DBUser:    getEnv("DB_USER", "gatherhub"),
		DBPass:    getEnv("DB_PASSWORD", "gatherhub"),
		DBName:    getEnv("DB_NAME", "gatherhub"),
		DBSSLMode: getEnv("DB_SSLMODE", "disable"),
		UploadDir: getEnv("UPLOAD_DIR", "./uploads"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
