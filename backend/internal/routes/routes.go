package routes

import (
	"github.com/gatherhub/backend/internal/handlers"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Register sets up all application routes
func Register(app *fiber.App, db *gorm.DB) {
	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db)

	// Root endpoint
	app.Get("/", healthHandler.Root)

	// Health check
	app.Get("/health", healthHandler.Health)

	// API v1 group (for future expansion)
	api := app.Group("/api/v1")
	_ = api // will be used in subsequent feature additions

	// 404 fallback
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Route not found",
		})
	})
}
