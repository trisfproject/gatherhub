package handlers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// HealthHandler handles health check and root endpoints
type HealthHandler struct {
	db *gorm.DB
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Root handles GET /
func (h *HealthHandler) Root(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "GatherHub API Running",
	})
}

// Health handles GET /health
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	// Verify database connection
	sqlDB, err := h.db.DB()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "error",
			"error":  "Database connection unavailable",
		})
	}

	if err := sqlDB.Ping(); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "error",
			"error":  "Database ping failed",
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
	})
}
