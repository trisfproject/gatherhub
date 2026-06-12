package routes

import (
	"github.com/gatherhub/backend/internal/handlers"
	"github.com/gatherhub/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Register sets up all application routes
func Register(app *fiber.App, db *gorm.DB, paymentUploadDir string) {
	// Initialize services
	eventService := services.NewEventService(db)
	participantService := services.NewParticipantService(db, paymentUploadDir)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db)
	eventHandler := handlers.NewEventHandler(eventService)
	participantHandler := handlers.NewParticipantHandler(participantService, eventService)

	// ---- Core Endpoints ----
	app.Get("/", healthHandler.Root)
	app.Get("/health", healthHandler.Health)

	// ---- API v1 ----
	api := app.Group("/api/v1")

	// Events
	api.Get("/events", eventHandler.List)
	api.Get("/events/:id", eventHandler.GetByID)

	// Registration
	api.Post("/events/:id/register", participantHandler.Register)

	// ---- HTMX Fragment Endpoints ----
	fragments := app.Group("/fragments")
	fragments.Get("/events", eventHandler.ListFragment)
	fragments.Get("/events/:id", eventHandler.GetFragment)

	// ---- 404 Fallback ----
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Route not found",
		})
	})
}
