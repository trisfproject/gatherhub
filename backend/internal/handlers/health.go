package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/trisfproject/gatherhub/internal/services"
	"gorm.io/gorm"
)

// HealthHandler handles health check and root endpoints
type HealthHandler struct {
	db            *gorm.DB
	healthService *services.HealthService
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler(db *gorm.DB, healthService *services.HealthService) *HealthHandler {
	return &HealthHandler{
		db:            db,
		healthService: healthService,
	}
}

// Root handles GET /
func (h *HealthHandler) Root(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "GatherHub API Running",
	})
}

// Health handles GET /health — returns a full health report as JSON
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	report := h.healthService.Report()

	status := fiber.StatusOK
	if report.Overall == services.StatusCritical {
		status = fiber.StatusServiceUnavailable
	} else if report.Overall == services.StatusWarning {
		status = fiber.StatusOK // still 200, but body indicates warning
	}

	return c.Status(status).JSON(fiber.Map{
		"status":   report.Overall,
		"uptime":   report.Uptime,
		"db":       report.DB,
		"storage":  report.Storage,
		"system":   report.System,
	})
}
