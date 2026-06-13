package middleware

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RequestLogger is a custom middleware that logs details of incoming HTTP requests.
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// Process request
		err := c.Next()
		
		latency := time.Since(start)
		status := c.Response().StatusCode()
		
		log.Printf("[REQ] method=%s path=%s status=%d latency=%s", c.Method(), c.Path(), status, latency)
		
		return err
	}
}
