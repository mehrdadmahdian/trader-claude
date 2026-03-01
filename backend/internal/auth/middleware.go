package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/trader-claude/backend/internal/models"
)

func RequireAuth(authSvc *AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization header"})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid authorization format"})
		}

		claims, err := authSvc.ValidateAccessToken(parts[1])
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("user_email", claims.Email)
		c.Locals("user_role", claims.Role)
		c.Locals("user_display_name", claims.DisplayName)

		return c.Next()
	}
}

func RequireRole(role models.UserRole) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("user_role").(string)
		if !ok || userRole != string(role) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "insufficient permissions"})
		}
		return c.Next()
	}
}

func OptionalAuth(authSvc *AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			claims, err := authSvc.ValidateAccessToken(parts[1])
			if err == nil {
				c.Locals("user_id", claims.UserID)
				c.Locals("user_email", claims.Email)
				c.Locals("user_role", claims.Role)
				c.Locals("user_display_name", claims.DisplayName)
			}
		}

		return c.Next()
	}
}

// GetUserID extracts user_id from Fiber context locals (set by RequireAuth).
func GetUserID(c *fiber.Ctx) int64 {
	uid, _ := c.Locals("user_id").(int64)
	return uid
}
