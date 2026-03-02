# Phase 14 — Security Audit & Hardening — Atomic Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Perform a comprehensive security audit and harden the trader-claude application: add input validation on all endpoints, security headers, extended rate limiting, security event logging, dependency vulnerability scanning, WebSocket hardening, Docker container hardening, and frontend XSS/storage audits.

**Architecture:** New `internal/validation/` package for input validation, new `internal/security/` package for event logging, new `internal/api/security.go` for HTTP security headers middleware. Dockerfile and docker-compose.yml updates for container hardening. Frontend CSP meta tag and storage/error audit.

**Tech Stack:** Go 1.24 (Fiber v2, `go-playground/validator/v10`, `govulncheck`), React 18, Docker.

**Execution strategy:** Each task = one focused action on ≤ 3 files. Give Haiku the task text + only the files listed under "Read first".

**Design doc:** `docs/plans/2026-02-27-phase14-security-design.md`

---

## BACKEND TASKS

---

### Task B1: Create internal/validation/validator.go + custom validators + tests

**Read first:** Nothing — standalone package.

**Files to create:**
- `backend/internal/validation/validator.go`
- `backend/internal/validation/validator_test.go`

---

**Step 1: Add `go-playground/validator/v10` dependency**

```bash
docker compose exec backend go get github.com/go-playground/validator/v10
```

---

**Step 2: Create `validator.go`**

```go
package validation

import (
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("safe_string", validateSafeString)
	validate.RegisterValidation("symbol", validateSymbol)
	validate.RegisterValidation("timeframe", validateTimeframe)
	validate.RegisterValidation("market", validateMarket)
}

func Validate(s interface{}) error {
	return validate.Struct(s)
}

func ValidateVar(field interface{}, tag string) error {
	return validate.Var(field, tag)
}

var symbolRegex = regexp.MustCompile(`^[A-Z0-9]{1,10}(/[A-Z0-9]{1,10})?$`)

var validTimeframes = map[string]bool{
	"1m": true, "5m": true, "15m": true, "30m": true,
	"1h": true, "4h": true, "1d": true, "1w": true,
}

var validMarkets = map[string]bool{
	"crypto": true, "stock": true, "forex": true,
}

func validateSafeString(fl validator.FieldLevel) bool {
	s := strings.ToLower(fl.Field().String())
	dangerous := []string{"<script", "javascript:", "\x00", "'; drop", "\" or 1=1", "union select"}
	for _, d := range dangerous {
		if strings.Contains(s, d) {
			return false
		}
	}
	return true
}

func validateSymbol(fl validator.FieldLevel) bool {
	return symbolRegex.MatchString(fl.Field().String())
}

func validateTimeframe(fl validator.FieldLevel) bool {
	return validTimeframes[fl.Field().String()]
}

func validateMarket(fl validator.FieldLevel) bool {
	return validMarkets[fl.Field().String()]
}
```

---

**Step 3: Create `validator_test.go`**

```go
package validation

import (
	"testing"
)

type testStruct struct {
	Name   string `validate:"required,safe_string"`
	Symbol string `validate:"required,symbol"`
	TF     string `validate:"required,timeframe"`
	Market string `validate:"required,market"`
}

func TestValidate_ValidInput(t *testing.T) {
	s := testStruct{Name: "My Backtest", Symbol: "BTC/USDT", TF: "1h", Market: "crypto"}
	if err := Validate(s); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidate_ScriptInjection(t *testing.T) {
	s := testStruct{Name: "<script>alert('xss')</script>", Symbol: "BTC/USDT", TF: "1h", Market: "crypto"}
	if err := Validate(s); err == nil {
		t.Error("expected error for script injection")
	}
}

func TestValidate_SQLInjection(t *testing.T) {
	s := testStruct{Name: "'; DROP TABLE users; --", Symbol: "BTC/USDT", TF: "1h", Market: "crypto"}
	if err := Validate(s); err == nil {
		t.Error("expected error for SQL injection")
	}
}

func TestValidate_InvalidSymbol(t *testing.T) {
	s := testStruct{Name: "Test", Symbol: "invalid symbol!", TF: "1h", Market: "crypto"}
	if err := Validate(s); err == nil {
		t.Error("expected error for invalid symbol")
	}
}

func TestValidate_ValidSymbolNoSlash(t *testing.T) {
	s := testStruct{Name: "Test", Symbol: "AAPL", TF: "1d", Market: "stock"}
	if err := Validate(s); err != nil {
		t.Errorf("expected valid for AAPL, got: %v", err)
	}
}

func TestValidate_InvalidTimeframe(t *testing.T) {
	s := testStruct{Name: "Test", Symbol: "BTC/USDT", TF: "2h", Market: "crypto"}
	if err := Validate(s); err == nil {
		t.Error("expected error for invalid timeframe")
	}
}

func TestValidate_InvalidMarket(t *testing.T) {
	s := testStruct{Name: "Test", Symbol: "BTC/USDT", TF: "1h", Market: "bonds"}
	if err := Validate(s); err == nil {
		t.Error("expected error for invalid market")
	}
}
```

---

**Step 4: Run tests**

```bash
docker compose exec backend go test ./internal/validation/... -v
```

Expected: all 7 tests PASS.

---

**Step 5: Commit**

```bash
git add backend/internal/validation/
git commit -m "feat(phase14): add input validation package with custom validators"
```

---

### Task B2: Add validation to existing API handlers

**Read first:**
- `backend/internal/api/backtest.go` (runBacktest handler)
- `backend/internal/api/alerts.go` (createAlert handler)
- `backend/internal/api/monitor_handler.go` (createMonitor handler)

**Files to modify:**
- `backend/internal/api/backtest.go`
- `backend/internal/api/alerts.go`
- `backend/internal/api/monitor_handler.go`
- `backend/internal/api/portfolio.go`

---

**Step 1: Define request structs with validation tags for each handler**

For each handler that accepts JSON body input, define a typed request struct with `validate` tags and call `validation.Validate()` after parsing the body. Return 400 with the validation error message if validation fails.

Example for backtest:

```go
type runBacktestRequest struct {
	StrategyName string  `json:"strategy_name" validate:"required,max=100,safe_string"`
	AdapterID    string  `json:"adapter_id" validate:"required,max=50"`
	Symbol       string  `json:"symbol" validate:"required,symbol"`
	Market       string  `json:"market" validate:"required,market"`
	Timeframe    string  `json:"timeframe" validate:"required,timeframe"`
	StartDate    string  `json:"start_date" validate:"required"`
	EndDate      string  `json:"end_date" validate:"required"`
	Capital      float64 `json:"initial_capital" validate:"required,gt=0"`
	Params       JSON    `json:"params"`
}
```

---

**Step 2: Test validation error responses**

Add test cases to existing handler tests that send invalid data and expect 400.

---

**Step 3: Commit**

```bash
git add backend/internal/api/
git commit -m "feat(phase14): add input validation to all mutation endpoints"
```

---

### Task B3: Create internal/api/security.go (security headers middleware)

**Read first:**
- `backend/cmd/server/main.go` (middleware setup)

**Files to create:**
- `backend/internal/api/security.go`
- `backend/internal/api/security_test.go`

**Files to modify:**
- `backend/cmd/server/main.go`

---

**Step 1: Create `security.go`**

```go
package api

import (
	"github.com/gofiber/fiber/v2"
)

func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "0")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self' ws://localhost:* wss://localhost:*; "+
				"font-src 'self'")
		if c.Protocol() == "https" {
			c.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		return c.Next()
	}
}
```

---

**Step 2: Create `security_test.go`**

```go
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
		"X-Frame-Options":       "DENY",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
	}

	for header, expected := range headers {
		got := resp.Header.Get(header)
		if got != expected {
			t.Errorf("expected %s=%s, got %s", header, expected, got)
		}
	}
}
```

---

**Step 3: Add `app.Use(api.SecurityHeaders())` in main.go**

Add after `app.Use(recover.New())` and before CORS middleware.

---

**Step 4: Run tests**

```bash
docker compose exec backend go test ./internal/api/... -v -run TestSecurity
```

---

**Step 5: Commit**

```bash
git add backend/internal/api/security.go backend/internal/api/security_test.go backend/cmd/server/main.go
git commit -m "feat(phase14): add HTTP security headers middleware"
```

---

### Task B4: CORS hardening + config startup validation

**Read first:**
- `backend/internal/config/config.go`
- `backend/cmd/server/main.go`

**Files to modify:**
- `backend/internal/config/config.go`

---

**Step 1: Add `Validate()` method to Config**

```go
func (cfg *Config) Validate() error {
	if cfg.App.Env == "production" {
		if cfg.App.JWTSecret == "" || cfg.App.JWTSecret == "change-me-in-production" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
		if len(cfg.App.JWTSecret) < 32 {
			return fmt.Errorf("JWT_SECRET must be at least 32 characters")
		}
		if cfg.DB.Password == "traderpassword" {
			return fmt.Errorf("default database password must not be used in production")
		}
		if cfg.CORS.Origins == "*" || cfg.CORS.Origins == "" {
			return fmt.Errorf("CORS_ORIGINS must be explicitly set in production")
		}
	}
	return nil
}
```

---

**Step 2: Call `cfg.Validate()` in main.go after `config.Load()`**

```go
if err := cfg.Validate(); err != nil {
    log.Fatalf("config validation failed: %v", err)
}
```

---

**Step 3: Add config validation tests**

Create `backend/internal/config/config_test.go`:

```go
func TestValidate_ProductionMissingJWT(t *testing.T) {
    cfg := &Config{App: AppConfig{Env: "production", JWTSecret: ""}}
    if err := cfg.Validate(); err == nil {
        t.Fatal("expected error for missing JWT secret")
    }
}

func TestValidate_DevelopmentNoErrors(t *testing.T) {
    cfg := &Config{App: AppConfig{Env: "development"}}
    if err := cfg.Validate(); err != nil {
        t.Fatalf("unexpected error for development: %v", err)
    }
}
```

---

**Step 4: Commit**

```bash
git add backend/internal/config/ backend/cmd/server/main.go
git commit -m "feat(phase14): add config startup validation for production environment"
```

---

### Task B5: Extend rate limiting to mutation endpoints

**Read first:**
- `backend/internal/api/routes.go`

**Files to modify:**
- `backend/internal/api/routes.go`
- `backend/cmd/server/main.go`

---

**Step 1: Add global rate limiter in main.go**

```go
import "github.com/gofiber/fiber/v2/middleware/limiter"

app.Use(limiter.New(limiter.Config{
    Max:        100,
    Expiration: 1 * time.Minute,
    KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
}))
```

---

**Step 2: Add tighter rate limiters to mutation routes in routes.go**

```go
mutationLimiter := limiter.New(limiter.Config{
    Max:        20,
    Expiration: 1 * time.Minute,
    KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
    LimitReached: func(c *fiber.Ctx) error {
        return c.Status(429).JSON(fiber.Map{"error": "rate limit exceeded"})
    },
})
```

Apply `mutationLimiter` to: `POST /backtest/run`, `POST /monitors`, `POST /alerts`, `POST /portfolios`.

---

**Step 3: Commit**

```bash
git add backend/internal/api/routes.go backend/cmd/server/main.go
git commit -m "feat(phase14): extend rate limiting to mutation endpoints"
```

---

### Task B6: Create internal/security/logger.go + integrate

**Read first:**
- `backend/internal/auth/middleware.go` (RequireAuth)
- `backend/internal/api/auth_handler.go` (login handler — from Phase 13)

**Files to create:**
- `backend/internal/security/logger.go`
- `backend/internal/security/logger_test.go`

**Files to modify:**
- `backend/internal/auth/middleware.go` (add security logging)
- `backend/internal/api/auth_handler.go` (add login event logging)

---

**Step 1: Create `logger.go`**

```go
package security

import (
	"log"
	"time"
)

type EventType string

const (
	EventLoginFailed    EventType = "login_failed"
	EventLoginSuccess   EventType = "login_success"
	EventPermissionDeny EventType = "permission_denied"
	EventRateLimit      EventType = "rate_limited"
	EventInvalidInput   EventType = "invalid_input"
	EventSuspicious     EventType = "suspicious_activity"
)

type SecurityEvent struct {
	Type      EventType
	IP        string
	UserID    int64
	UserAgent string
	Path      string
	Detail    string
	Timestamp time.Time
}

func LogEvent(event SecurityEvent) {
	event.Timestamp = time.Now()
	log.Printf("[SECURITY] type=%s ip=%s user_id=%d path=%s detail=%s",
		event.Type, event.IP, event.UserID, event.Path, event.Detail)
}
```

---

**Step 2: Add `security.LogEvent()` calls to auth handler and middleware**

- On failed login: `security.LogEvent(SecurityEvent{Type: EventLoginFailed, IP: c.IP(), ...})`
- On successful login: `security.LogEvent(SecurityEvent{Type: EventLoginSuccess, ...})`
- On RequireAuth 401: `security.LogEvent(SecurityEvent{Type: EventPermissionDeny, ...})`

---

**Step 3: Run tests**

```bash
docker compose exec backend go test ./internal/security/... -v
docker compose exec backend go test ./internal/auth/... -v
```

---

**Step 4: Commit**

```bash
git add backend/internal/security/ backend/internal/auth/ backend/internal/api/
git commit -m "feat(phase14): add security event logging for auth and access control"
```

---

### Task B7: Secrets audit — scan for hardcoded secrets

**Read first:**
- `backend/.env.example`
- `backend/internal/config/config.go`

---

**Step 1: Grep codebase for common secret patterns**

Scan for:
- Hardcoded passwords, tokens, API keys in Go/TypeScript source
- Strings like `password =`, `secret =`, `api_key =` with literal values
- Base64-encoded strings that look like tokens

---

**Step 2: Ensure `.env` is in `.gitignore`**

Verify `.env` is listed in `.gitignore`. If not, add it.

---

**Step 3: Verify no secrets in committed code**

```bash
git log --all --diff-filter=A -- '*.env' '*.key' '*.pem'
```

---

**Step 4: Commit any fixes**

```bash
git commit -m "fix(phase14): remove hardcoded secrets and ensure .env in .gitignore"
```

---

### Task B8: Add govulncheck + npm audit to Makefile

**Read first:**
- `Makefile` (existing targets)

**Files to modify:**
- `Makefile`

---

**Step 1: Add security targets**

```makefile
security-check-backend:
	docker compose exec backend go install golang.org/x/vuln/cmd/govulncheck@latest
	docker compose exec backend govulncheck ./...

security-check-frontend:
	docker compose exec frontend npm audit --audit-level=high

security-check: security-check-backend security-check-frontend
```

---

**Step 2: Run checks and fix any critical findings**

```bash
make security-check
```

---

**Step 3: Commit**

```bash
git add Makefile
git commit -m "feat(phase14): add security vulnerability scanning to Makefile"
```

---

### Task B9: WebSocket hardening

**Read first:**
- `backend/internal/api/routes.go` (WebSocket routes)

**Files to modify:**
- `backend/internal/api/routes.go`

---

**Step 1: Add origin validation to WS upgrade middleware**

```go
app.Use("/ws", func(c *fiber.Ctx) error {
    if websocket.IsWebSocketUpgrade(c) {
        origin := c.Get("Origin")
        allowedOrigins := strings.Split(cfg.CORS.Origins, ",")
        allowed := false
        for _, o := range allowedOrigins {
            if strings.TrimSpace(o) == origin {
                allowed = true
                break
            }
        }
        if !allowed && origin != "" {
            return c.Status(403).JSON(fiber.Map{"error": "origin not allowed"})
        }
        return c.Next()
    }
    return fiber.ErrUpgradeRequired
})
```

---

**Step 2: Configure message size limits on WebSocket connections**

Add `ReadBufferSize` and `WriteBufferSize` to websocket.Config in each `websocket.New()` call:

```go
websocket.New(hub.ServeWS, websocket.Config{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
})
```

---

**Step 3: Commit**

```bash
git add backend/internal/api/routes.go
git commit -m "feat(phase14): harden WebSocket with origin validation and message size limits"
```

---

### Task B10: Docker container hardening

**Read first:**
- `backend/Dockerfile`
- `docker-compose.yml`

**Files to modify:**
- `backend/Dockerfile`
- `docker-compose.yml`

---

**Step 1: Add non-root user to backend Dockerfile**

Add before the `CMD` or `ENTRYPOINT`:

```dockerfile
RUN adduser -D -g '' appuser
USER appuser
```

---

**Step 2: Add security options to docker-compose.yml**

```yaml
services:
  backend:
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    tmpfs:
      - /tmp
```

---

**Step 3: Verify containers start and pass health check**

```bash
make down && make up
make health
```

---

**Step 4: Commit**

```bash
git add backend/Dockerfile docker-compose.yml
git commit -m "feat(phase14): harden Docker containers with non-root user and security options"
```

---

## FRONTEND TASKS

---

### Task F1: Frontend XSS audit

**Read first:**
- All files in `frontend/src/pages/`
- All files in `frontend/src/components/`

---

**Step 1: Search for unsafe raw HTML injection patterns**

Grep for patterns that bypass React's auto-escaping, such as `innerHTML` usage or equivalent React APIs. Verify all user-supplied data flows through React's JSX rendering which auto-escapes.

---

**Step 2: If any unsafe patterns found, replace with safe alternatives**

- Use DOMPurify + the `sanitize` function for any unavoidable raw HTML
- Or restructure to use React components instead of raw HTML strings

---

**Step 3: Commit fixes if any**

---

### Task F2: LocalStorage audit

**Read first:**
- `frontend/src/api/client.ts`
- `frontend/src/stores/index.ts`

---

**Step 1: Remove `token` from localStorage references**

The axios interceptor in `client.ts` currently reads `localStorage.getItem('token')`. After Phase 13, this should read from the auth Zustand store instead. Verify this is updated.

---

**Step 2: Audit all localStorage usage**

Ensure only non-sensitive data is stored:
- `trader-theme` (theme preference) — OK
- `sidebar-collapsed` — OK
- Chart/indicator settings — OK
- Tokens, passwords, API keys — MUST NOT be stored

---

**Step 3: Commit fixes if any**

---

### Task F3: Add CSP meta tag to index.html

**Read first:**
- `frontend/index.html`

**Files to modify:**
- `frontend/index.html`

---

**Step 1: Add Content-Security-Policy meta tag**

Inside `<head>`:

```html
<meta http-equiv="Content-Security-Policy"
  content="default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' ws://localhost:* wss://localhost:*; font-src 'self'">
```

---

**Step 2: Test that the app still works**

```bash
make frontend-dev
# Open browser, verify no CSP violations in console
```

---

**Step 3: Commit**

```bash
git add frontend/index.html
git commit -m "feat(phase14): add Content-Security-Policy meta tag"
```

---

### Task F4: Sanitize error display in UI components

**Read first:**
- `frontend/src/api/client.ts` (error interceptor)

**Files to modify:**
- `frontend/src/api/client.ts`
- Error display components across pages

---

**Step 1: Ensure error interceptor sanitizes messages**

```ts
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const message = error.response?.data?.error
    if (message && typeof message === 'string') {
      // Only show generic messages, strip internal details
      const safeMessage = message.length > 200 ? 'An error occurred' : message
      return Promise.reject(new Error(safeMessage))
    }
    return Promise.reject(new Error('An unexpected error occurred'))
  },
)
```

---

**Step 2: Verify error toast/alert components don't render raw HTML**

Ensure all error messages are displayed as text content, not as HTML.

---

**Step 3: Commit**

```bash
git add frontend/src/
git commit -m "feat(phase14): sanitize error display and limit message length"
```

---

**Final Verification:**

```bash
make backend-test
make frontend-lint
make frontend-test
make security-check
```

---

## Summary

| Task | Files | Description |
|---|---|---|
| B1 | validation/validator.go | Input validation package + custom validators + tests |
| B2 | api/*.go | Add validation tags to all mutation handlers |
| B3 | api/security.go, main.go | HTTP security headers middleware |
| B4 | config/config.go, main.go | Startup config validation for production |
| B5 | routes.go, main.go | Global + per-route rate limiting |
| B6 | security/logger.go, auth handlers | Security event logging |
| B7 | Codebase scan | Secrets audit — remove hardcoded values |
| B8 | Makefile | govulncheck + npm audit targets |
| B9 | routes.go | WebSocket origin validation + message limits |
| B10 | Dockerfile, docker-compose.yml | Non-root user + security options |
| F1 | Frontend components | XSS audit for unsafe HTML injection |
| F2 | api/client.ts, stores | LocalStorage audit — remove sensitive data |
| F3 | index.html | Content-Security-Policy meta tag |
| F4 | api/client.ts, error components | Sanitize error display |
