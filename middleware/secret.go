package middleware

import (
	"master-panel-api/config"

	"github.com/gofiber/fiber/v2"
)

// RequireWorkerSecret ensures requests from Cloudflare Worker or external gateway have the valid secret
func RequireWorkerSecret() fiber.Handler {
	return func(c *fiber.Ctx) error {
		secretHeader := c.Get("X-Master-Secret")
		secretQuery := c.Query("secret")

		expectedSecret := config.AppConfig.WorkerSecret

		if secretHeader != expectedSecret && secretQuery != expectedSecret {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Unauthorized: Invalid or missing X-Master-Secret",
			})
		}

		return c.Next()
	}
}
