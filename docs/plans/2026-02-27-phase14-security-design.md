# Phase 14 — Security Audit & Hardening: Design Document

**Date:** 2026-02-27
**Status:** Draft
**Requires:** Phase 13 (Authentication) complete.

---

## Overview

Phase 14 performs a comprehensive security audit of the trader-claude codebase and hardens both the backend and frontend against common web vulnerabilities. This phase covers input validation, SQL injection prevention, XSS mitigation, CORS hardening, secrets management, dependency auditing, HTTP security headers, logging/monitoring for security events, and Docker container hardening.

---

## Decisions Made

| Topic | Decision |
|---|---|
| Input validation | Server-side validation on all API endpoints using `go-playground/validator/v10` |
| SQL injection | Already mitigated by GORM parameterized queries; audit confirms no raw SQL concatenation |
| XSS | Server-side: HTML-escape all user-supplied strings. Frontend: React auto-escapes; audit for unsafe raw HTML injection patterns |
| CORS | Tighten to explicit allowed origins from config, no wildcards in production |
| Secrets | Move all secrets to env vars, validate at startup, add `.env` to `.gitignore`, scan for leaked secrets |
| Rate limiting | Extend from login-only (Phase 13) to all mutation endpoints (configurable per-route) |
| Security headers | Fiber middleware: Helmet-equivalent (CSP, X-Frame-Options, X-Content-Type-Options, etc.) |
| Dependency audit | Go: `govulncheck`, npm: `npm audit` — run in CI |
| Logging | Security event log: failed logins, permission denials, rate limits, suspicious patterns |
| Docker | Non-root user in containers, read-only filesystem where possible, no unnecessary capabilities |
| API key security | Provider API keys (Binance, NewsAPI, etc.) validated at startup, never logged or returned in API responses |
| WebSocket security | Connection-level auth (from Phase 13), message size limits, origin validation |

---

## Section 1: Input Validation Layer

### New file: `internal/validation/validator.go`

```go
package validation

import (
    "github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
    validate = validator.New()
    validate.RegisterValidation("safe_string", validateSafeString)
    validate.RegisterValidation("symbol", validateSymbol)
}

func Validate(s interface{}) error {
    return validate.Struct(s)
}
```

### Request structs with validation tags

All API request bodies get explicit struct definitions with validation:

```go
type CreateBacktestRequest struct {
    StrategyName string  `json:"strategy_name" validate:"required,max=100"`
    Symbol       string  `json:"symbol" validate:"required,symbol,max=20"`
    Market       string  `json:"market" validate:"required,oneof=crypto stock forex"`
    Timeframe    string  `json:"timeframe" validate:"required,oneof=1m 5m 15m 1h 4h 1d"`
    StartDate    string  `json:"start_date" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
    EndDate      string  `json:"end_date" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
    Capital      float64 `json:"initial_capital" validate:"required,gt=0,lte=1000000000"`
    Params       JSON    `json:"params"`
}
```

### Custom validators

```go
// validateSafeString rejects strings with common injection patterns
func validateSafeString(fl validator.FieldLevel) bool {
    s := fl.Field().String()
    dangerous := []string{"<script", "javascript:", "\x00", "'; DROP", "\" OR 1=1"}
    for _, d := range dangerous {
        if strings.Contains(strings.ToLower(s), strings.ToLower(d)) {
            return false
        }
    }
    return true
}

// validateSymbol ensures valid trading pair format
func validateSymbol(fl validator.FieldLevel) bool {
    s := fl.Field().String()
    matched, _ := regexp.MatchString(`^[A-Z0-9]{1,10}(/[A-Z0-9]{1,10})?$`, s)
    return matched
}
```

---

## Section 2: HTTP Security Headers

### New middleware: `internal/api/security.go`

```go
func SecurityHeaders() fiber.Handler {
    return func(c *fiber.Ctx) error {
        c.Set("X-Content-Type-Options", "nosniff")
        c.Set("X-Frame-Options", "DENY")
        c.Set("X-XSS-Protection", "0") // modern browsers: CSP preferred
        c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        c.Set("Content-Security-Policy",
            "default-src 'self'; "+
            "script-src 'self'; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data: blob:; "+
            "connect-src 'self' ws://localhost:* wss://localhost:*; "+
            "font-src 'self'")
        if c.Secure() {
            c.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
        }
        return c.Next()
    }
}
```

---

## Section 3: CORS Hardening

### Update CORS config

```go
app.Use(cors.New(cors.Config{
    AllowOrigins:     cfg.CORS.Origins, // "https://yourapp.com"
    AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
    AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
    AllowCredentials: true,
    MaxAge:           3600,
}))
```

Add validation at startup:
```go
if cfg.App.Env == "production" && (cfg.CORS.Origins == "*" || cfg.CORS.Origins == "") {
    log.Fatal("CORS_ORIGINS must be explicitly set in production")
}
```

---

## Section 4: Rate Limiting Extension

### Global rate limiter + per-route overrides

```go
// Global: 100 requests per minute per IP
app.Use(limiter.New(limiter.Config{
    Max:        100,
    Expiration: 1 * time.Minute,
    KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
}))

// Mutation-heavy endpoints: tighter limits
mutationLimiter := limiter.New(limiter.Config{
    Max:        20,
    Expiration: 1 * time.Minute,
    KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
})

protected.Post("/backtest/run", mutationLimiter, bh.runBacktest)
protected.Post("/monitors", mutationLimiter, mnh.createMonitor)
```

---

## Section 5: Security Event Logging

### New file: `internal/security/logger.go`

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

Integration points:
- Auth handler: log `LoginFailed`, `LoginSuccess`
- RequireAuth middleware: log `PermissionDeny` on 401/403
- Rate limiter: log `RateLimit` when triggered
- Validation: log `InvalidInput` on suspicious patterns

---

## Section 6: Secrets Management & Startup Validation

### Config validation at startup

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
    }
    return nil
}
```

### Secrets audit checklist

- `.env` is in `.gitignore`
- No hardcoded secrets in source code
- API keys (Binance, NewsAPI) never logged
- Database credentials never in API responses
- JWT secret sufficiently long and random

---

## Section 7: Dependency Vulnerability Scanning

### Go vulnerability check

```bash
docker compose exec backend go install golang.org/x/vuln/cmd/govulncheck@latest
docker compose exec backend govulncheck ./...
```

### npm audit

```bash
docker compose exec frontend npm audit --audit-level=high
```

### Makefile targets

```makefile
security-check:
	docker compose exec backend govulncheck ./...
	docker compose exec frontend npm audit --audit-level=high

security-fix:
	docker compose exec frontend npm audit fix
```

---

## Section 8: WebSocket Hardening

```go
// Message size limit (1MB)
app.Get("/ws", websocket.New(hub.ServeWS, websocket.Config{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}))

// Origin validation in WS upgrade middleware
app.Use("/ws", func(c *fiber.Ctx) error {
    if websocket.IsWebSocketUpgrade(c) {
        origin := c.Get("Origin")
        if !isAllowedOrigin(origin, cfg.CORS.Origins) {
            return c.Status(403).JSON(fiber.Map{"error": "origin not allowed"})
        }
        return c.Next()
    }
    return fiber.ErrUpgradeRequired
})
```

---

## Section 9: Docker Container Hardening

### Backend Dockerfile updates

```dockerfile
# Run as non-root user
RUN adduser -D -g '' appuser
USER appuser

# Read-only filesystem where possible
# Data directories explicitly mounted as writable
```

### Docker Compose security settings

```yaml
services:
  backend:
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
    cap_drop:
      - ALL
```

### MySQL hardening

```yaml
  mysql:
    environment:
      MYSQL_ROOT_HOST: '%' # restrict in production
    command: --default-authentication-plugin=caching_sha2_password
```

---

## Section 10: Frontend Security

### XSS Prevention Audit

- Verify no raw HTML injection patterns are used in React components
- Ensure user-supplied data is never injected as unescaped HTML
- `react-markdown` (Phase 10): renders safely by default; audit any custom renderers
- Use DOMPurify for any cases where raw HTML rendering is unavoidable

### LocalStorage Audit

- Remove `token` from `localStorage` (Phase 13 uses Zustand in-memory)
- Only persist non-sensitive data: theme preference, sidebar state, chart settings
- Never store API keys, tokens, or passwords

### Content Security Policy in index.html

```html
<meta http-equiv="Content-Security-Policy"
  content="default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' ws://localhost:* wss://localhost:*; font-src 'self'">
```

### Frontend error handling

- Never display raw error messages from the backend that might leak internal details
- Sanitize error messages before display
- Log errors to console only in development mode

---

## Implementation Order

```
B1: internal/validation/validator.go + custom validators + tests       (no deps)
B2: Add validation tags to all existing request structs                 (needs B1)
B3: internal/api/security.go (security headers middleware)              (no deps)
B4: CORS hardening + startup config validation                          (no deps, parallel B3)
B5: Rate limiter extension to mutation endpoints                        (no deps, parallel B3-B4)
B6: internal/security/logger.go + integrate into auth + middleware      (needs Phase 13)
B7: Secrets audit — scan codebase for hardcoded secrets                 (no deps)
B8: govulncheck + npm audit setup + Makefile targets                   (no deps)
B9: WebSocket hardening (message limits, origin check)                  (no deps)
B10: Docker container hardening                                          (no deps)
F1: Frontend XSS audit — check for unsafe raw HTML injection            (no deps)
F2: LocalStorage audit — remove sensitive data                          (needs Phase 13)
F3: Add CSP meta tag to index.html                                      (no deps)
F4: Sanitize error display in UI components                             (no deps)
```

---

## Testing Requirements

| Task | Tests |
|---|---|
| B1 | Custom validators: safe_string rejects script tags, symbol validates format |
| B2 | Existing handler tests updated to include validation errors |
| B3 | Security headers present in all responses |
| B4 | Production startup fails without proper CORS config |
| B5 | Rate limiter returns 429 after exceeding threshold |
| B6 | Security events logged for failed login, permission deny |
| B7 | No hardcoded secrets in codebase (grep-based scan) |
| B8 | govulncheck runs without critical findings |
| B9 | Oversized WS messages rejected, invalid origin blocked |
| F1 | No unsafe raw HTML injection patterns in React components |
