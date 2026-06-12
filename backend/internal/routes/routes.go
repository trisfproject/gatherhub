package routes

import (
	"log"

	"github.com/gatherhub/backend/internal/handlers"
	"github.com/gatherhub/backend/internal/middleware"
	"github.com/gatherhub/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"gorm.io/gorm"
)

// Register sets up all application routes.
// storage holds the resolved paths for all upload directories.
func Register(app *fiber.App, db *gorm.DB, storageService *services.StorageService, adminUsername, adminPassword string, store *session.Store) {
	// ── Services ──────────────────────────────────────────────
	eventService := services.NewEventService(db)
	participantService := services.NewParticipantService(db, storageService)

	adminService := services.NewAdminService(db)
	if err := adminService.SeedDefaultAdmin(); err != nil {
		log.Printf("Warning: failed to seed default admin: %v", err)
	}

	// ── Handlers ──────────────────────────────────────────────
	healthHandler := handlers.NewHealthHandler(db)

	pageHandler, err := handlers.NewPageHandler(eventService, participantService)
	if err != nil {
		log.Fatalf("Failed to initialise page handler: %v", err)
	}

	adminHandler, err := handlers.NewAdminHandler(participantService, eventService, store, adminService, storageService)
	if err != nil {
		log.Fatalf("Failed to initialise admin handler: %v", err)
	}

	// ── Public Page Routes (SSR) ───────────────────────────────
	app.Get("/", pageHandler.Landing)
	app.Get("/register", pageHandler.RegisterPage)
	app.Post("/register", pageHandler.RegisterSubmit)
	app.Get("/register/success", pageHandler.Success)
	app.Get("/event/:slug", pageHandler.EventBySlug)

	// ── Infrastructure ────────────────────────────────────────
	app.Get("/health", healthHandler.Health)

	// ── JSON API (v1) ─────────────────────────────────────────
	api := app.Group("/api/v1")

	eventHandler := handlers.NewEventHandler(eventService)
	api.Get("/events", eventHandler.List)
	api.Get("/events/:id", eventHandler.GetByID)

	// HTMX fragment endpoints
	fragments := app.Group("/fragments")
	fragments.Get("/events", eventHandler.ListFragment)
	fragments.Get("/events/:id", eventHandler.GetFragment)

	// ── Admin Routes ──────────────────────────────────────────
	app.Get("/admin/login", adminHandler.LoginPage)
	app.Post("/admin/login", adminHandler.LoginSubmit)
	app.Get("/admin/logout", adminHandler.Logout)

	admin := app.Group("/admin", middleware.AdminAuth(store))
	admin.Get("/dashboard", adminHandler.Dashboard)
	admin.Get("/participants", adminHandler.ParticipantList)
	admin.Get("/participants/export", adminHandler.ExportParticipants)
	admin.Get("/participants/:id", adminHandler.ParticipantDetail)
	admin.Post("/participants/:id/status", adminHandler.UpdateStatus)

	// Event management routes
	admin.Get("/events", adminHandler.EventList)
	admin.Get("/events/create", adminHandler.EventCreatePage)
	admin.Post("/events/create", adminHandler.EventCreateSubmit)
	admin.Get("/events/:id", adminHandler.EventDetail)
	admin.Get("/events/:id/edit", adminHandler.EventEditPage)
	admin.Post("/events/:id/edit", adminHandler.EventEditSubmit)
	admin.Post("/events/:id/status", adminHandler.EventUpdateStatus)
	admin.Delete("/events/:id", adminHandler.EventDelete)
	admin.Post("/events/:id/delete", adminHandler.EventDelete)

	// Admin management & settings (SUPER_ADMIN only)
	superAdmin := admin.Group("/", middleware.RequireRole(store, "SUPER_ADMIN"))
	superAdmin.Get("/admins", adminHandler.AdminList)
	superAdmin.Get("/admins/create", adminHandler.AdminCreatePage)
	superAdmin.Post("/admins/create", adminHandler.AdminCreateSubmit)
	superAdmin.Get("/admins/:id/edit", adminHandler.AdminEditPage)
	superAdmin.Post("/admins/:id/edit", adminHandler.AdminEditSubmit)
	superAdmin.Delete("/admins/:id", adminHandler.AdminDelete)
	superAdmin.Post("/admins/:id/delete", adminHandler.AdminDelete)
	superAdmin.Get("/settings", adminHandler.SystemSettingsPage)
	superAdmin.Post("/settings", adminHandler.SystemSettingsSubmit)

	app.Get("/admin", func(c *fiber.Ctx) error {
		return c.Redirect("/admin/dashboard", fiber.StatusSeeOther)
	})

	// ── 404 Fallback ──────────────────────────────────────────
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Route not found",
		})
	})
}
