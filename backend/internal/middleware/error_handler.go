package middleware

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CustomErrorHandler intercepts all errors, hides detailed database/system traces from the client,
// and returns a user-friendly error page, JSON, or HTMX fragment based on the request.
func CustomErrorHandler(c *fiber.Ctx, err error) error {
	// Status code defaults to 500
	code := fiber.StatusInternalServerError

	// Retrieve the custom status code if it's a *fiber.Error
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}

	// Log the actual internal error with context details
	log.Printf("ERROR: [Method: %s] [Path: %s] [Status: %d] - Details: %v", c.Method(), c.Path(), code, err)

	// Determine client preference (JSON vs HTML vs HTMX)
	accept := c.Get("Accept")
	isJSON := strings.Contains(accept, "application/json") || strings.HasPrefix(c.Path(), "/api/")
	isHTMX := c.Get("HX-Request") == "true"

	if isJSON {
		msg := "Internal Server Error"
		if code < 500 {
			msg = err.Error()
		}
		return c.Status(code).JSON(fiber.Map{
			"error": msg,
		})
	}

	// Indonesia message fallback for HTML
	msg := "Terjadi kesalahan internal pada server. Silakan hubungi administrator jika masalah berlanjut."
	if code < 500 {
		msg = err.Error()
	}

	if isHTMX {
		// HTMX partial error response
		html := fmt.Sprintf(`
<div class="alert-error" style="background:rgba(239,68,68,.1);border:1.5px solid rgba(239,68,68,.3);border-radius:14px;padding:16px 20px;color:#fca5a5;font-weight:600;font-family:sans-serif;">
  ⚠️ %s
</div>`, template.HTMLEscapeString(msg))
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(code).SendString(html)
	}

	// Full HTML error page
	html := fmt.Sprintf(`<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"/><title>Error — GatherHub</title>
<script src="https://cdn.tailwindcss.com"></script></head>
<body class="bg-[#07071a] text-white min-h-screen flex items-center justify-center">
<div class="text-center px-6">
  <div class="text-6xl mb-6">⚠️</div>
  <h1 class="text-2xl font-bold mb-3">Oops!</h1>
  <p class="text-white/60 mb-8 max-w-md mx-auto">%s</p>
  <a href="/" class="inline-flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 font-semibold px-6 py-3 rounded-xl text-sm transition-colors">
    Kembali ke Halaman Utama
  </a>
</div></body></html>`, template.HTMLEscapeString(msg))
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(code).SendString(html)
}
