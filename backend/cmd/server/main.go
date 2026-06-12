package main

import (
	"log"
	"os"

	"github.com/gatherhub/backend/internal/config"
	"github.com/gatherhub/backend/internal/database"
	"github.com/gatherhub/backend/internal/routes"
	"github.com/gatherhub/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Ensure storage directories exist
	for _, dir := range []string{cfg.UploadDir, cfg.PaymentUploadDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Warning: could not create directory %s: %v", dir, err)
		}
	}

	// Connect to database
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run auto migrations
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed sample event if the events table is empty
	eventService := services.NewEventService(db)
	if err := eventService.SeedSampleIfEmpty(); err != nil {
		log.Printf("Warning: could not seed sample event: %v", err)
	}

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName:     "GatherHub v1.0",
		BodyLimit:   11 * 1024 * 1024, // 11 MB — accommodates 10 MB file + form fields
		ProxyHeader: fiber.HeaderXForwardedFor,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} ${method} ${path} (${latency})\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, HX-Request, HX-Current-URL, HX-Target, HX-Trigger",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))

	// Serve uploaded payment proofs at /payments/*
	app.Static("/payments", cfg.PaymentUploadDir, fiber.Static{
		MaxAge:   86400,
		Compress: false,
	})

	// Register all page and API routes
	routes.Register(app, db, cfg.PaymentUploadDir)

	// Start server
	addr := ":" + cfg.AppPort
	log.Printf("✓ GatherHub running on http://localhost%s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
