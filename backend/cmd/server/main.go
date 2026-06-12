package main

import (
	"log"
	"time"

	"github.com/gatherhub/backend/internal/config"
	"github.com/gatherhub/backend/internal/database"
	"github.com/gatherhub/backend/internal/routes"
	"github.com/gatherhub/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
)

func main() {
	// ── Load configuration ────────────────────────────────────
	cfg := config.Load()

	// ── Initialize storage ────────────────────────────────────
	// All runtime uploads live under STORAGE_PATH, outside the Git repository.
	storage := config.NewStorageConfig(cfg.StoragePath)

	if err := storage.Init(); err != nil {
		log.Fatalf("Failed to initialize storage directories: %v", err)
	}

	// Warn if the legacy in-repo storage directory still exists.
	// The legacy path is two levels up from the backend binary: ../../storage
	storage.CheckLegacy("../")

	// ── Connect to database ───────────────────────────────────
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// ── Run auto migrations ───────────────────────────────────
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
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
	app.Static("/payments", storage.Payments, fiber.Static{
		MaxAge:   86400,
		Compress: false,
	})
	// Event banners → /events/*
	app.Static("/events", storage.Events, fiber.Static{
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

	// ── Register all routes ───────────────────────────────────
	routes.Register(app, db, storage, cfg.AdminUsername, cfg.AdminPassword, store)

	// ── Start server ──────────────────────────────────────────
	addr := ":" + cfg.AppPort
	log.Printf("✓ GatherHub running on http://localhost%s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
