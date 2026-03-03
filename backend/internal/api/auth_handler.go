package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/security"
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

	// Issue a refresh token cookie immediately after registration
	_, refreshToken, _, loginErr := h.authSvc.Login(c.Context(), req.Email, req.Password, c.Get("User-Agent"), c.IP())
	if loginErr == nil {
		setRefreshCookie(c, refreshToken)
	}
	// Don't fail registration if refresh token creation fails — access token is still valid

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
		security.LogEvent(security.SecurityEvent{
			Type:      security.EventLoginFailed,
			IP:        ip,
			Path:      c.Path(),
			UserAgent: userAgent,
			Detail:    "login attempt failed",
		})
		msg := err.Error()
		if msg == "invalid email or password" || msg == "account is disabled" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": msg})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "login failed"})
	}

	security.LogEvent(security.SecurityEvent{
		Type:      security.EventLoginSuccess,
		IP:        ip,
		Path:      c.Path(),
		UserID:    user.ID,
		UserAgent: userAgent,
	})

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
		msg := err.Error()
		if msg == "invalid refresh token" || msg == "refresh token expired" || msg == "user not found" || msg == "account is disabled" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": msg})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token refresh failed"})
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
	user, err := h.authSvc.GetUser(c.Context(), auth.GetUserID(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(user)
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

	user, err := h.authSvc.UpdateProfile(c.Context(), auth.GetUserID(c), req.DisplayName, req.Password, req.CurrentPassword)
	if err != nil {
		msg := err.Error()
		switch msg {
		case "current password is incorrect":
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": msg})
		case "user not found":
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": msg})
		default:
			if strings.Contains(msg, "password must") || strings.Contains(msg, "password policy") {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update profile"})
		}
	}
	return c.JSON(user)
}

func setRefreshCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HTTPOnly: true,
		Secure:   true,
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
