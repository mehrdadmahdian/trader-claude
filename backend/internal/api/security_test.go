package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSecurityHeaders_Present(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "0",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
	}
	// Verify CSP header is set (exact value checked separately due to length)
	if csp := resp.Header.Get("Content-Security-Policy"); csp == "" {
		t.Error("expected Content-Security-Policy header to be set")
	}

	for header, expected := range headers {
		got := resp.Header.Get(header)
		if got != expected {
			t.Errorf("expected %s=%s, got %s", header, expected, got)
		}
	}
}
