package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/trisfproject/gatherhub/internal/config"
	"github.com/trisfproject/gatherhub/internal/database"
	"github.com/trisfproject/gatherhub/internal/routes"
	"github.com/trisfproject/gatherhub/internal/services"
)

func main() {
	// ── Load configuration ────────────────────────────────────
	cfg := config.Load()

	// ── Connect to database ───────────────────────────────────
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// ── Run auto migrations ───────────────────────────────────
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// ── Load Settings Service ─────────────────────────────────
	settingsService := services.NewSettingsService(db)
	if err := settingsService.Load(); err != nil {
		log.Fatalf("Failed to load settings service: %v", err)
	}

	// ── Initialize storage ────────────────────────────────────
	// All runtime uploads live under STORAGE_PATH, outside the Git repository.
	storageService, err := services.NewStorageService(cfg.StoragePath, settingsService)
	if err != nil {
		log.Fatalf("Failed to initialize storage service: %v", err)
	}

	// ── Seed sample event if events table is empty ────────────
	eventService := services.NewEventService(db)
	if err := eventService.SeedSampleIfEmpty(); err != nil {
		log.Printf("Warning: could not seed sample event: %v", err)
	}

	// ── Initialize Fiber app ──────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:     "GatherHub v1.0",
		BodyLimit:   11 * 1024 * 1024, // 11 MB — accommodates 10 MB file + form fields
		ProxyHeader: fiber.HeaderXForwardedFor,
	})

	// ── Global middleware ─────────────────────────────────────
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} ${method} ${path} (${latency})\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, HX-Request, HX-Current-URL, HX-Target, HX-Trigger",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))

	// ── Serve uploaded files as static assets ─────────────────
	// Payment proofs → /payments/*
	app.Static("/payments", storageService.GetPaymentsPath(), fiber.Static{
		MaxAge:   86400,
		Compress: false,
	})
	// Event banners → /events/*
	app.Static("/events", storageService.GetEventsPath(), fiber.Static{
		MaxAge:   86400,
		Compress: false,
	})

	// ── Initialize session store ──────────────────────────────
	store := session.New(session.Config{
		Expiration:     8 * time.Hour,
		CookieHTTPOnly: true,
		CookieSecure:   cfg.AppEnv == "production",
		CookieSameSite: "Lax",
	})

	// ── Initialize backup service ─────────────────────────────
	backupService := services.NewBackupService(cfg)

	// ── Register all routes ───────────────────────────────────
	routes.Register(app, db, storageService, cfg.AdminUsername, cfg.AdminPassword, store, cfg.SessionSecret, settingsService, backupService)

	// ── Start server ──────────────────────────────────────────
	addr := ":" + cfg.AppPort
	log.Printf("✓ GatherHub running on http://localhost%s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
