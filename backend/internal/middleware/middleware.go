package middleware

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

// RequestID adds a unique X-Request-ID header to each request
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateID()
		}
		c.Set("X-Request-ID", requestID)
		c.Locals("requestID", requestID)
		return c.Next()
	}
}

// AdminAuth returns a middleware that requires an authenticated admin session.
// On failure it redirects to /admin/login (HTML) or returns 401 (JSON/HTMX).
func AdminAuth(store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return c.Redirect("/admin/login", fiber.StatusSeeOther)
		}

		authenticated, _ := sess.Get("admin_authenticated").(bool)
		if !authenticated {
			// HTMX / JSON clients get 401, browsers get redirect
			if c.Get("HX-Request") == "true" || c.Get("Accept") == "application/json" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Unauthorized",
				})
			}
			return c.Redirect("/admin/login", fiber.StatusSeeOther)
		}

		// Propagate admin username and role to downstream handlers
		adminUser, _ := sess.Get("admin_username").(string)
		adminRole, _ := sess.Get("admin_role").(string)

		c.Locals("admin_username", adminUser)
		c.Locals("admin_role", adminRole)

		return c.Next()
	}
}

// RequireRole checks if the authenticated admin has one of the allowed roles.
// On failure it returns a 403 response.
func RequireRole(store *session.Store, allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return c.Redirect("/admin/login", fiber.StatusSeeOther)
		}

		role, _ := sess.Get("admin_role").(string)
		for _, allowed := range allowedRoles {
			if role == allowed {
				return c.Next()
			}
		}

		// Forbidden
		if c.Get("HX-Request") == "true" || c.Get("Accept") == "application/json" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Forbidden",
			})
		}

		html := fmt.Sprintf(`<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"/><title>Akses Ditolak — GatherHub</title>
<script src="https://cdn.tailwindcss.com"></script></head>
<body class="bg-[#07071a] text-white min-h-screen flex items-center justify-center">
<div class="text-center px-6">
  <div class="text-6xl mb-6">🚫</div>
  <h1 class="text-2xl font-bold mb-3">Akses Ditolak</h1>
  <p class="text-white/60 mb-8 font-semibold">Anda tidak memiliki izin (peran %s) untuk mengakses halaman ini.</p>
  <a href="/admin/dashboard" class="inline-flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 font-semibold px-6 py-3 rounded-xl text-sm transition-colors">
    Kembali ke Dashboard
  </a>
</div></body></html>`, role)

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusForbidden).SendString(html)
	}
}

func generateID() string {
	src := rand.NewSource(time.Now().UnixNano())
	r := rand.New(src)
	return fmt.Sprintf("%d-%d", time.Now().UnixMilli(), r.Int63())
}
