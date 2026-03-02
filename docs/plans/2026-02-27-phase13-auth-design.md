# Phase 13 — Authentication, Authorization & Permissions: Design Document

**Date:** 2026-02-27
**Status:** Draft

---

## Overview

Phase 13 adds a complete security layer to trader-claude: **JWT-based authentication** (register, login, refresh, logout), **role-based access control** (RBAC) with admin/user roles, and **resource-level ownership enforcement** so users can only access their own data. The system is currently single-user with `user_id` fields present on some models; this phase activates multi-user support end-to-end.

---

## Decisions Made

| Topic | Decision |
|---|---|
| Auth token type | JWT (HS256) via `golang-jwt/jwt/v5` — stateless, compact, industry-standard |
| Token storage (backend) | Stateless access tokens (15 min TTL) + opaque refresh tokens stored in MySQL |
| Token storage (frontend) | Access token in memory (Zustand), refresh token in `httpOnly` secure cookie |
| Password hashing | bcrypt via `golang.org/x/crypto/bcrypt` (cost=12) |
| User model | New `users` table with email (unique), password hash, role, active flag |
| Refresh token model | New `refresh_tokens` table (token hash, user_id, expires_at, revoked) |
| RBAC strategy | Simple role field (`admin`, `user`) on User model + Fiber middleware |
| Resource ownership | Fiber middleware extracts `user_id` from JWT, passes via `c.Locals("user_id")`. All queries scoped by `user_id`. |
| Session invalidation | Logout revokes all refresh tokens for the user. Optional "revoke all sessions" endpoint. |
| Password policy | Min 8 chars, at least 1 uppercase, 1 lowercase, 1 digit |
| Rate limiting | Login endpoint rate-limited to 5 attempts per minute per IP via Fiber limiter middleware |
| WebSocket auth | Token sent as query param `?token=` on WS upgrade, validated before connection |
| Admin capability | Admin can list users, toggle active status, view all resources (future) |
| CSRF protection | Not needed — JWT in Authorization header (not cookie-based auth) |
| Email verification | Deferred — out of scope for this phase |
| OAuth providers | Deferred — out of scope for this phase |

---

## Section 1: New DB Models

### `users` table

```go
// internal/models/models.go
type UserRole string

const (
    UserRoleAdmin UserRole = "admin"
    UserRoleUser  UserRole = "user"
)

type User struct {
    ID           int64          `gorm:"primaryKey;autoIncrement" json:"id"`
    Email        string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"email"`
    PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
    DisplayName  string         `gorm:"type:varchar(100)" json:"display_name"`
    Role         UserRole       `gorm:"type:varchar(20);not null;default:'user'" json:"role"`
    Active       bool           `gorm:"default:true" json:"active"`
    LastLoginAt  *time.Time     `json:"last_login_at,omitempty"`
    CreatedAt    time.Time      `json:"created_at"`
    UpdatedAt    time.Time      `json:"updated_at"`
    DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }
```

### `refresh_tokens` table

```go
type RefreshToken struct {
    ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID    int64     `gorm:"not null;index" json:"user_id"`
    TokenHash string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"-"`
    ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
    Revoked   bool      `gorm:"default:false" json:"revoked"`
    UserAgent string    `gorm:"type:varchar(512)" json:"user_agent"`
    IP        string    `gorm:"type:varchar(45)" json:"ip"`
    CreatedAt time.Time `json:"created_at"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }
```

### Existing models — add `UserID`

All resource models that currently lack `UserID` need it added:

```go
// Add to: Backtest, Portfolio, Alert, WatchList, Monitor, Setting, Notification
UserID int64 `gorm:"not null;default:1;index" json:"user_id"`
```

This is safe for existing data: `default:1` ensures migration fills the column for existing rows.

---

## Section 2: Backend — `internal/auth/` Package

### File layout

```
internal/auth/
  auth.go           AuthService: Register, Login, RefreshToken, Logout, ValidateToken
  auth_test.go      Unit tests for all auth flows
  jwt.go            JWT token creation and parsing helpers
  jwt_test.go       JWT generation + validation tests
  password.go       HashPassword, ComparePassword (bcrypt wrappers)
  password_test.go  Bcrypt tests
  middleware.go      Fiber middleware: RequireAuth, RequireRole, ExtractUserID
  middleware_test.go Middleware tests
```

### AuthService

```go
type AuthService struct {
    db        *gorm.DB
    jwtSecret []byte
    accessTTL time.Duration // 15 minutes
    refreshTTL time.Duration // 7 days
}

func NewAuthService(db *gorm.DB, jwtSecret string) *AuthService

// Register creates a new user (validates email uniqueness + password policy)
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*models.User, error)

// Login validates credentials and returns access + refresh tokens
func (s *AuthService) Login(ctx context.Context, email, password, userAgent, ip string) (accessToken string, refreshToken string, user *models.User, err error)

// RefreshToken validates a refresh token and issues new access + refresh tokens (rotation)
func (s *AuthService) RefreshToken(ctx context.Context, oldRefreshToken, userAgent, ip string) (accessToken string, newRefreshToken string, err error)

// Logout revokes all refresh tokens for the user
func (s *AuthService) Logout(ctx context.Context, userID int64) error

// ValidateAccessToken parses and validates a JWT access token, returns claims
func (s *AuthService) ValidateAccessToken(tokenStr string) (*Claims, error)
```

### JWT Claims

```go
type Claims struct {
    UserID      int64    `json:"user_id"`
    Email       string   `json:"email"`
    Role        string   `json:"role"`
    DisplayName string   `json:"display_name"`
    jwt.RegisteredClaims
}
```

### Password Utilities

```go
func HashPassword(password string) (string, error)   // bcrypt cost=12
func ComparePassword(hash, password string) error     // bcrypt.CompareHashAndPassword
func ValidatePasswordPolicy(password string) error    // min 8, upper, lower, digit
```

### Fiber Middleware

```go
// RequireAuth extracts and validates JWT from Authorization header.
// Sets c.Locals("user_id"), c.Locals("user_role"), c.Locals("user_email").
func RequireAuth(authSvc *AuthService) fiber.Handler

// RequireRole checks that the authenticated user has the given role.
// Must be used AFTER RequireAuth.
func RequireRole(role models.UserRole) fiber.Handler

// OptionalAuth attempts to extract user from token but doesn't fail if missing.
// Useful for endpoints that work for both authenticated and anonymous users.
func OptionalAuth(authSvc *AuthService) fiber.Handler
```

---

## Section 3: Auth API Endpoints

### New file: `internal/api/auth_handler.go`

```
POST /api/v1/auth/register    → { user, access_token }  + Set-Cookie: refresh_token
POST /api/v1/auth/login       → { user, access_token }  + Set-Cookie: refresh_token
POST /api/v1/auth/refresh     → { access_token }         + Set-Cookie: refresh_token (rotated)
POST /api/v1/auth/logout      → { success: true }        + Clear-Cookie: refresh_token
GET  /api/v1/auth/me          → { user }                 (requires auth)
PUT  /api/v1/auth/me          → { user }                 (update display_name, password)
```

### Register request/response

```json
// Request
{ "email": "user@example.com", "password": "SecurePass1", "display_name": "John" }

// Response 201
{
  "user": { "id": 1, "email": "user@example.com", "display_name": "John", "role": "user" },
  "access_token": "eyJhbG..."
}
// Set-Cookie: refresh_token=opaque-token; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth; Max-Age=604800
```

### Login request/response

```json
// Request
{ "email": "user@example.com", "password": "SecurePass1" }

// Response 200
{
  "user": { "id": 1, "email": "user@example.com", "display_name": "John", "role": "user" },
  "access_token": "eyJhbG..."
}
```

### Error responses

```json
// 400 — validation
{ "error": "password must be at least 8 characters with uppercase, lowercase, and digit" }

// 401 — bad credentials
{ "error": "invalid email or password" }

// 409 — duplicate email
{ "error": "email already registered" }

// 429 — rate limited
{ "error": "too many login attempts, try again later" }
```

---

## Section 4: Admin API Endpoints

### New file: `internal/api/admin_handler.go`

```
GET    /api/v1/admin/users              → [{ user }]     (admin only)
PATCH  /api/v1/admin/users/:id/toggle   → { user }       (admin only — toggle active)
PATCH  /api/v1/admin/users/:id/role     → { user }       (admin only — change role)
```

---

## Section 5: Securing Existing Routes

### Middleware application strategy

```go
// Public (no auth required)
app.Get("/health", health.check)
v1.Post("/auth/register", authH.register)
v1.Post("/auth/login", authH.login)
v1.Post("/auth/refresh", authH.refresh)

// Protected (require valid JWT)
protected := v1.Group("", auth.RequireAuth(authSvc))
protected.Post("/auth/logout", authH.logout)
protected.Get("/auth/me", authH.me)
protected.Put("/auth/me", authH.updateMe)

// All resource routes go under `protected`
protected.Get("/markets", mh.listAdapters)
protected.Post("/backtest/run", bh.runBacktest)
// ... all existing routes moved to `protected` group

// Admin routes
admin := protected.Group("/admin", auth.RequireRole(models.UserRoleAdmin))
admin.Get("/users", adminH.listUsers)
admin.Patch("/users/:id/toggle", adminH.toggleUser)
admin.Patch("/users/:id/role", adminH.changeRole)
```

### Resource ownership enforcement

Every handler that queries user-owned resources adds `.Where("user_id = ?", c.Locals("user_id"))`:

```go
// Before (no auth)
db.Find(&backtests)

// After (with auth)
userID := c.Locals("user_id").(int64)
db.Where("user_id = ?", userID).Find(&backtests)
```

Create/update operations set `UserID` from the JWT claims:

```go
backtest.UserID = c.Locals("user_id").(int64)
db.Create(&backtest)
```

---

## Section 6: WebSocket Authentication

```go
// On WS upgrade, extract token from query param
app.Get("/ws", websocket.New(func(conn *websocket.Conn) {
    token := conn.Query("token")
    claims, err := authSvc.ValidateAccessToken(token)
    if err != nil {
        conn.WriteJSON(fiber.Map{"type": "error", "data": fiber.Map{"message": "unauthorized"}})
        conn.Close()
        return
    }
    // Store user info on connection for scoped subscriptions
    hub.ServeWSAuthenticated(conn, claims.UserID)
}))
```

---

## Section 7: Frontend

### New TypeScript types

```ts
// types/index.ts
export interface User {
  id: number
  email: string
  display_name: string
  role: 'admin' | 'user'
  active: boolean
  last_login_at?: string
  created_at: string
}

export interface AuthResponse {
  user: User
  access_token: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  email: string
  password: string
  display_name: string
}
```

### Auth Zustand Store

```ts
// stores/authStore.ts
interface AuthState {
  user: User | null
  accessToken: string | null
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, displayName: string) => Promise<void>
  logout: () => Promise<void>
  refreshToken: () => Promise<void>
  setUser: (user: User | null) => void
}
```

### New Pages

```
frontend/src/pages/Login.tsx        Email + password form, link to register
frontend/src/pages/Register.tsx     Email + password + display name form, link to login
```

### Auth Route Guard

```ts
// components/auth/ProtectedRoute.tsx
// Wraps routes that require authentication
// Redirects to /login if not authenticated
// Attempts token refresh on 401
```

### Axios Interceptor Updates

```ts
// api/client.ts
// Request interceptor: attach accessToken from Zustand store (not localStorage)
// Response interceptor: on 401, attempt token refresh, retry original request
// If refresh fails, redirect to /login
```

### New Files

```
frontend/src/api/auth.ts                  login(), register(), refresh(), logout(), getMe(), updateMe()
frontend/src/stores/authStore.ts          Auth Zustand store
frontend/src/pages/Login.tsx              Login page
frontend/src/pages/Register.tsx           Register page
frontend/src/components/auth/ProtectedRoute.tsx
frontend/src/hooks/useAuth.ts             Convenience hooks
```

---

## Section 8: Auto-Refresh Flow

```
1. User opens app → authStore.refreshToken() called
2. POST /api/v1/auth/refresh (cookie sent automatically)
3. If success → store new access_token in memory, user is logged in
4. If 401 → redirect to /login
5. On any API 401 → interceptor calls refreshToken()
6. If refresh succeeds → retry original request with new token
7. If refresh fails → logout + redirect to /login
```

---

## Section 9: First User / Seed Admin

On first startup with no users:
- `make seed` creates an admin user: `admin@trader-claude.local` / `AdminPass1`
- Or: `POST /api/v1/auth/register` creates the first user as `admin` role (if no users exist)

---

## Implementation Order

```
B1: Add User + RefreshToken models → autoMigrate                    (no deps)
B2: Add UserID column to existing models                             (needs B1)
B3: internal/auth/password.go + tests                                (no deps)
B4: internal/auth/jwt.go + tests                                     (no deps, parallel B3)
B5: internal/auth/auth.go (AuthService) + tests                      (needs B1, B3, B4)
B6: internal/auth/middleware.go + tests                               (needs B5)
B7: api/auth_handler.go + routes                                     (needs B5, B6)
B8: api/admin_handler.go + routes                                    (needs B6, B7)
B9: Secure existing routes with RequireAuth middleware               (needs B6)
B10: Add user_id scoping to all existing handlers                    (needs B9)
B11: WebSocket authentication                                        (needs B6)
B12: Login rate limiter                                               (needs B7)
F1: types + api/auth.ts                                              (no deps)
F2: stores/authStore.ts                                              (needs F1)
F3: Update api/client.ts interceptors                                (needs F2)
F4: Login + Register pages                                           (needs F2)
F5: ProtectedRoute component                                        (needs F2)
F6: Wire ProtectedRoute into App.tsx router                          (needs F4, F5)
F7: Update Sidebar/TopBar with user info + logout                    (needs F2)
```

---

## Testing Requirements

| Task | Tests |
|---|---|
| B3 | Hash + compare success, compare failure, policy validation (6 cases) |
| B4 | JWT generate + parse, expired token, invalid signature, missing claims |
| B5 | Register (success, duplicate email, bad password), Login (success, bad password, inactive user), Refresh (success, revoked, expired), Logout |
| B6 | Middleware: valid token passes, missing token 401, expired token 401, wrong role 403 |
| B7 | Auth API integration: full register → login → refresh → logout flow |
| B10 | Resource isolation: user A cannot see user B's backtests |
| F4 | Login form validation, successful login redirects, error display |
| F5 | ProtectedRoute redirects unauthenticated users to /login |
