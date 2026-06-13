package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/trisfproject/gatherhub/internal/config"
	"github.com/trisfproject/gatherhub/internal/database"
	"github.com/trisfproject/gatherhub/internal/middleware"
	"github.com/trisfproject/gatherhub/internal/routes"
	"github.com/trisfproject/gatherhub/internal/services"
)

func main() {
	// ── Load configuration ────────────────────────────────────
	cfg := config.Load()

	// ── Display application banner ────────────────────────────
	fmt.Printf(`
=============================================
   GatherHub %s
   Build Time: %s
   Git Commit: %s
=============================================
`, services.AppVersion, services.BuildTime, services.GitCommit)

	// ── Configuration validation ──────────────────────────────
	if missing := cfg.Validate(); len(missing) > 0 {
		log.Fatalf("FATAL: Startup configuration validation failed. The following required environment variables are missing or empty: %s", strings.Join(missing, ", "))
	}

	// ── Connect to database ───────────────────────────────────
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// ── Database connectivity validation ──────────────────────
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("FATAL: Failed to get database connection handle: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("FATAL: Database connection validation failed: %v", err)
	}

	// Ensure PostgreSQL enum has all required values before migrations/updates
	db.Exec("ALTER TYPE participant_status ADD VALUE IF NOT EXISTS 'REGISTERED'")
	db.Exec("ALTER TYPE participant_status ADD VALUE IF NOT EXISTS 'WAITLIST'")
	db.Exec("ALTER TYPE participant_status ADD VALUE IF NOT EXISTS 'CHECKED_IN'")

	// ── Run auto migrations ───────────────────────────────────
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Update existing data for waiting list setting and status
	db.Exec("UPDATE events SET enable_waiting_list = true WHERE enable_waiting_list IS NULL")
	db.Exec("UPDATE participants SET status = 'REGISTERED' WHERE status = 'PENDING'")
	db.Exec("UPDATE events SET enable_sponsors = false WHERE enable_sponsors IS NULL")

	// ── Load Settings Service ─────────────────────────────────
	settingsService := services.NewSettingsService(db)
	if err := settingsService.Load(); err != nil {
		log.Fatalf("Failed to load settings service: %v", err)
	}

	// ── Initialize storage ────────────────────────────────────
	// Validate storage path writability eagerly
	if err := os.MkdirAll(cfg.StoragePath, 0o755); err != nil {
		log.Fatalf("FATAL: STORAGE_PATH (%s) could not be created/accessed: %v", cfg.StoragePath, err)
	}
	tempFile, err := os.CreateTemp(cfg.StoragePath, ".startup_write_test_*")
	if err != nil {
		log.Fatalf("FATAL: STORAGE_PATH (%s) is not writable: %v", cfg.StoragePath, err)
	}
	tempFile.Close()
	os.Remove(tempFile.Name())

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
		AppName:      fmt.Sprintf("GatherHub %s", services.AppVersion),
		BodyLimit:    11 * 1024 * 1024, // 11 MB — accommodates 10 MB file + form fields
		ProxyHeader:  fiber.HeaderXForwardedFor,
		ErrorHandler: middleware.CustomErrorHandler,
	})

	// ── Global middleware ─────────────────────────────────────
	// Recover middleware must be first in the stack to recover from all panics
	app.Use(recover.New())
	// Custom request logger tracking method, path, status, and duration
	app.Use(middleware.RequestLogger())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, HX-Request, HX-Current-URL, HX-Target, HX-Trigger",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))

	// ── Serve uploaded files as static assets ─────────────────
	app.Static("/payments", storageService.GetPaymentsPath(), fiber.Static{
		MaxAge:   86400,
		Compress: false,
	})
	app.Static("/events", storageService.GetEventsPath(), fiber.Static{
		MaxAge:   86400,
		Compress: false,
	})
	app.Static("/sponsors", storageService.GetSponsorsPath(), fiber.Static{
		MaxAge:   86400,
		Compress: false,
	})

	// ── Initialize session store ──────────────────────────────
	store := session.New(session.Config{
		Expiration:     8 * time.Hour,
		CookieName:     "gatherhub_session_id",
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSecure:   cfg.AppEnv == "production",
		CookieSameSite: "Lax",
	})

	// ── Initialize backup service ─────────────────────────────
	backupService := services.NewBackupService(cfg)

	// ── Register all routes ───────────────────────────────────
	routes.Register(app, db, storageService, store, cfg.SessionSecret, settingsService, backupService)

	// ── Start server & handle Graceful Shutdown ───────────────
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.AppPort
		log.Printf("✓ GatherHub running on http://localhost%s", addr)
		if err := app.Listen(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	sig := <-sigChan
	log.Printf("Shutting down gracefully (received signal %s)...", sig.String())

	// Give active connections 15 seconds to finish
	if err := app.ShutdownWithTimeout(15 * time.Second); err != nil {
		log.Printf("Warning: Shutdown error: %v", err)
	}
	log.Println("GatherHub stopped.")
}
