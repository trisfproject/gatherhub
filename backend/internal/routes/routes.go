package routes

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/trisfproject/gatherhub/internal/handlers"
	"github.com/trisfproject/gatherhub/internal/middleware"
	"github.com/trisfproject/gatherhub/internal/services"
	"gorm.io/gorm"
)

// Register sets up all application routes.
// storage holds the resolved paths for all upload directories.
func Register(app *fiber.App, db *gorm.DB, storageService *services.StorageService, adminUsername, adminPassword string, store *session.Store, sessionSecret string, settingsService *services.SettingsService, backupService *services.BackupService) {
	// ── Services ──────────────────────────────────────────────
	auditLogService := services.NewAuditLogService(db)
	checkinService := services.NewCheckinService(db, sessionSecret)
	eventService := services.NewEventService(db)
	participantService := services.NewParticipantService(db, storageService)
	notificationService := services.NewNotificationService(db, auditLogService, settingsService)
	broadcastService := services.NewBroadcastService(db, notificationService, auditLogService)

	adminService := services.NewAdminService(db)
	if err := adminService.SeedDefaultAdmin(); err != nil {
		log.Printf("Warning: failed to seed default admin: %v", err)
	}

	// ── Handlers ──────────────────────────────────────────────
	healthHandler := handlers.NewHealthHandler(db)

	pageHandler, err := handlers.NewPageHandler(eventService, participantService, notificationService, checkinService, settingsService)
	if err != nil {
		log.Fatalf("Failed to initialise page handler: %v", err)
	}

	adminHandler, err := handlers.NewAdminHandler(participantService, eventService, store, adminService, storageService, notificationService, auditLogService, checkinService, settingsService, backupService, broadcastService)
	if err != nil {
		log.Fatalf("Failed to initialise admin handler: %v", err)
	}
	// ── Maintenance Interceptor ───────────────────────────────
	app.Use(func(c *fiber.Ctx) error {
		path := c.Path()
		// Allow admin routes, static assets, and health check to pass through
		if strings.HasPrefix(path, "/admin") ||
			strings.HasPrefix(path, "/payments") ||
			strings.HasPrefix(path, "/events") ||
			path == "/health" {
			return c.Next()
		}

		if settingsService.GetBool("maintenance_mode") {
			return pageHandler.RenderMaintenance(c)
		}
		return c.Next()
	})

	// ── Public Page Routes (SSR) ───────────────────────────────
	app.Get("/", pageHandler.Landing)
	app.Get("/register", pageHandler.RegisterPage)
	app.Post("/register", pageHandler.RegisterSubmit)
	app.Get("/register/success", pageHandler.Success)
	app.Get("/event/:slug", pageHandler.EventBySlug)
	app.Get("/checkin", pageHandler.CheckinPage)
	app.Post("/checkin", pageHandler.CheckinSubmit)

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
	admin.Get("/participants/:id/qr", adminHandler.ParticipantQRPage)
	admin.Get("/notifications", adminHandler.NotificationList)
	admin.Get("/audit-logs", adminHandler.AuditLogList)

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

	// Backup management routes
	admin.Get("/backups", adminHandler.BackupsPage)
	admin.Post("/backups/create", adminHandler.CreateBackupSubmit)
	admin.Get("/backups/download/:filename", adminHandler.DownloadBackup)
	admin.Post("/backups/restore/:filename", adminHandler.RestoreBackupSubmit)
	admin.Post("/backups/upload", adminHandler.UploadRestoreBackup)
	admin.Post("/backups/delete/:filename", adminHandler.DeleteBackupSubmit)

	// Check-in routes
	admin.Get("/checkin", adminHandler.CheckinPage)
	admin.Post("/checkin/:participant_id", adminHandler.CheckinSubmit)
	admin.Get("/attendance", adminHandler.AttendanceDashboard)

	// Broadcast center routes
	admin.Get("/broadcasts", adminHandler.BroadcastList)
	admin.Get("/broadcasts/create", adminHandler.BroadcastCreatePage)
	admin.Post("/broadcasts/create", adminHandler.BroadcastCreateSubmit)
	admin.Post("/broadcasts/preview", adminHandler.BroadcastPreview)
	admin.Get("/broadcasts/count-recipients", adminHandler.BroadcastCountRecipients)
	admin.Get("/broadcasts/:id", adminHandler.BroadcastDetail)

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
