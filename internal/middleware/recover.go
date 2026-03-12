package middleware

import (
	"fmt"
	"log/slog"
	"github.com/gofiber/fiber/v2"
)

func Recover(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered",
					"err", fmt.Sprintf("%v", r),
					"method", c.Method(),
					"path", c.Path(),
				)
				err = c.Status(fiber.StatusInternalServerError).
					JSON(fiber.Map{"error": "internal server error"})
			}
		}()
		return c.Next()
	}
}
