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

		// Propagate admin username to downstream handlers
		adminUser, _ := sess.Get("admin_username").(string)
		c.Locals("admin_username", adminUser)

		return c.Next()
	}
}

func generateID() string {
	src := rand.NewSource(time.Now().UnixNano())
	r := rand.New(src)
	return fmt.Sprintf("%d-%d", time.Now().UnixMilli(), r.Int63())
}
