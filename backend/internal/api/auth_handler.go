package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/trader-claude/backend/internal/auth"
)

type authHandler struct {
	authSvc *auth.AuthService
}

func newAuthHandler(authSvc *auth.AuthService) *authHandler {
	return &authHandler{authSvc: authSvc}
}

func (h *authHandler) register(c *fiber.Ctx) error {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email and password are required"})
	}

	user, accessToken, err := h.authSvc.Register(c.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "Duplicate") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "UNIQUE") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "email already registered"})
		}
		if strings.Contains(msg, "password policy") || strings.Contains(msg, "password must") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "registration failed"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user":         user,
		"access_token": accessToken,
	})
}

func (h *authHandler) login(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userAgent := c.Get("User-Agent")
	ip := c.IP()

	accessToken, refreshToken, user, err := h.authSvc.Login(c.Context(), req.Email, req.Password, userAgent, ip)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	setRefreshCookie(c, refreshToken)

	return c.JSON(fiber.Map{
		"user":         user,
		"access_token": accessToken,
	})
}

func (h *authHandler) refresh(c *fiber.Ctx) error {
	oldToken := c.Cookies("refresh_token")
	if oldToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing refresh token"})
	}

	userAgent := c.Get("User-Agent")
	ip := c.IP()

	accessToken, newRefresh, err := h.authSvc.RefreshToken(c.Context(), oldToken, userAgent, ip)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	setRefreshCookie(c, newRefresh)

	return c.JSON(fiber.Map{"access_token": accessToken})
}

func (h *authHandler) logout(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	if err := h.authSvc.Logout(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "logout failed"})
	}
	clearRefreshCookie(c)
	return c.JSON(fiber.Map{"success": true})
}

func (h *authHandler) me(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"id":           auth.GetUserID(c),
		"email":        c.Locals("user_email"),
		"display_name": c.Locals("user_display_name"),
		"role":         c.Locals("user_role"),
	})
}

func (h *authHandler) updateMe(c *fiber.Ctx) error {
	var req struct {
		DisplayName     string `json:"display_name"`
		Password        string `json:"password"`
		CurrentPassword string `json:"current_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Return current user info from token claims (full update would require DB access)
	return c.JSON(fiber.Map{
		"id":           auth.GetUserID(c),
		"email":        c.Locals("user_email"),
		"display_name": c.Locals("user_display_name"),
		"role":         c.Locals("user_role"),
	})
}

func setRefreshCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HTTPOnly: true,
		SameSite: "Strict",
		MaxAge:   7 * 24 * 60 * 60,
	})
}

func clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HTTPOnly: true,
		MaxAge:   -1,
	})
}
