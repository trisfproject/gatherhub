package routes

import (
	"log"

	"github.com/gatherhub/backend/internal/handlers"
	"github.com/gatherhub/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Register sets up all application routes
func Register(app *fiber.App, db *gorm.DB, paymentUploadDir string) {
	// ── Services ──────────────────────────────────────────
	eventService := services.NewEventService(db)
	participantService := services.NewParticipantService(db, paymentUploadDir)

	// ── Handlers ──────────────────────────────────────────
	healthHandler := handlers.NewHealthHandler(db)

	pageHandler, err := handlers.NewPageHandler(eventService, participantService)
	if err != nil {
		log.Fatalf("Failed to initialise page handler: %v", err)
	}

	// ── Public Page Routes (SSR) ───────────────────────────
	// Landing page — shows event details with "Daftar Sekarang" CTA
	app.Get("/", pageHandler.Landing)

	// Registration form — shows payment info + form
	app.Get("/register", pageHandler.RegisterPage)

	// Registration submit — saves participant, redirects to success
	app.Post("/register", pageHandler.RegisterSubmit)

	// Success page — shows registration number + status
	app.Get("/register/success", pageHandler.Success)

	// ── Infrastructure ────────────────────────────────────
	app.Get("/health", healthHandler.Health)

	// ── JSON API (v1) ─────────────────────────────────────
	api := app.Group("/api/v1")

	// Event handlers (JSON)
	eventHandler := handlers.NewEventHandler(eventService)
	api.Get("/events", eventHandler.List)
	api.Get("/events/:id", eventHandler.GetByID)

	// HTMX fragment endpoints
	fragments := app.Group("/fragments")
	fragments.Get("/events", eventHandler.ListFragment)
	fragments.Get("/events/:id", eventHandler.GetFragment)

	// ── 404 Fallback ──────────────────────────────────────
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Route not found",
		})
	})
}
