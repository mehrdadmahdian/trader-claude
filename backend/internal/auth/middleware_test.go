package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/trader-claude/backend/internal/models"
)

func TestRequireAuth_ValidToken(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	token, _ := GenerateAccessToken(svc.jwtSecret, 1, "a@b.com", "user", "A", 15*time.Minute)

	app := fiber.New()
	app.Use(RequireAuth(svc))
	app.Get("/test", func(c *fiber.Ctx) error {
		uid := GetUserID(c)
		return c.JSON(fiber.Map{"user_id": uid})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	app := fiber.New()
	app.Use(RequireAuth(svc))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	token, _ := GenerateAccessToken(svc.jwtSecret, 1, "a@b.com", "user", "A", -1*time.Hour)

	app := fiber.New()
	app.Use(RequireAuth(svc))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireRole_Admin(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	adminToken, _ := GenerateAccessToken(svc.jwtSecret, 1, "admin@b.com", "admin", "Admin", 15*time.Minute)
	userToken, _ := GenerateAccessToken(svc.jwtSecret, 2, "user@b.com", "user", "User", 15*time.Minute)

	app := fiber.New()
	app.Use(RequireAuth(svc))
	app.Get("/admin", RequireRole(models.UserRoleAdmin), func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	// Admin should pass
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("admin expected 200, got %d", resp.StatusCode)
	}

	// Regular user should get 403
	req2 := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req2.Header.Set("Authorization", "Bearer "+userToken)
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != 403 {
		t.Errorf("user expected 403, got %d", resp2.StatusCode)
	}
}
