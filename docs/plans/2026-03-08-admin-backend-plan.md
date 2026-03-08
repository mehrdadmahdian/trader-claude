# Admin Dashboard — Backend: RBAC + API

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend the backend with a full RBAC system (Roles, Permissions, Audit Log) and a comprehensive admin API. After this phase, admins can manage users, roles, and permissions via API. Every protected route can require fine-grained permissions.

**Architecture:** RBAC sits on top of the existing binary `admin/user` role system — backwards compatible. New models: `Role`, `Permission`, `RolePermission` (join), `AuditLog`. A `RequirePermission` middleware reads the user's role → looks up its permissions → enforces. Default roles and permissions are seeded on startup.

**Tech Stack:** Go/Fiber v2, GORM, MySQL 8.0 (existing stack — no new dependencies)

---

## Existing state (do not break)

- `models.User` has `Role UserRole` field (`"admin"` | `"user"`) — keep this
- 3 admin routes exist: `GET /admin/users`, `PATCH /admin/users/:id/toggle`, `PATCH /admin/users/:id/role`
- `auth.RequireRole(models.UserRoleAdmin)` middleware already works
- All existing behaviour must remain unchanged

---

## RBAC Design

### Permission format: `resource:action`

```
users:read      users:write     users:delete
roles:read      roles:write     roles:delete
backtest:read   backtest:run    backtest:delete
portfolio:read  portfolio:write
alerts:read     alerts:write
monitor:read    monitor:write   monitor:run
news:read
settings:read   settings:write
audit:read
system:read
```

### Default roles (seeded, not deletable by API)

| Role | Permissions |
|------|------------|
| `superadmin` | `*` (all — wildcard) |
| `admin` | all except `system:read` write-destructive ops |
| `trader` | backtest:*, portfolio:*, alerts:*, monitor:*, news:read, settings:read |
| `viewer` | backtest:read, portfolio:read, news:read |

### How permission check works

1. JWT middleware (already exists) sets `userID` and `role` string in Fiber locals
2. `RequirePermission("users:write")` middleware reads `role` string → queries `roles` + `role_permissions` + `permissions` → checks if permission name is in set
3. `superadmin` bypasses all checks (wildcard)
4. Cache permission sets in Redis keyed by role name (TTL 5 min) to avoid DB hit per request

---

## Task B1: Add RBAC models to models.go

**Files:**
- Modify: `backend/internal/models/models.go`

**Step 1: Add these structs** at the end of `models.go`:

```go
// --- RBAC ---

// Permission is an atomic capability in `resource:action` format.
type Permission struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`   // "users:write"
	Resource    string    `gorm:"type:varchar(50);not null;index" json:"resource"`       // "users"
	Action      string    `gorm:"type:varchar(50);not null" json:"action"`               // "write"
	Description string    `gorm:"type:varchar(255)" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Role is a named set of permissions.
type Role struct {
	ID          int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string       `gorm:"type:varchar(50);not null;uniqueIndex" json:"name"`
	Description string       `gorm:"type:varchar(255)" json:"description"`
	IsSystem    bool         `gorm:"default:false" json:"is_system"`  // system roles cannot be deleted
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// AuditLog records admin actions for accountability.
type AuditLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64     `gorm:"index" json:"user_id"`
	UserEmail  string    `gorm:"type:varchar(255)" json:"user_email"`
	Action     string    `gorm:"type:varchar(100);not null;index" json:"action"`       // "user.created", "role.updated"
	Resource   string    `gorm:"type:varchar(50);not null;index" json:"resource"`      // "user", "role"
	ResourceID string    `gorm:"type:varchar(50)" json:"resource_id"`
	Details    JSON      `gorm:"type:json" json:"details"`
	IPAddress  string    `gorm:"type:varchar(45)" json:"ip_address"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}
```

**Step 2: Verify auto-migrate creates tables**
```bash
make up
make health
make db-shell
# In MySQL:
SHOW TABLES LIKE 'permissions';
SHOW TABLES LIKE 'roles';
SHOW TABLES LIKE 'role_permissions';
SHOW TABLES LIKE 'audit_logs';
```
Expected: all 4 tables exist.

**Step 3: Commit**
```bash
git add backend/internal/models/models.go
git commit -m "feat(models): add RBAC models — Permission, Role, AuditLog + role_permissions join"
```

---

## Task B2: Create RBAC seeder

**Files:**
- Create: `backend/internal/admin/seed.go`

**Step 1: Create the package and file**

```go
package admin

import (
	"log"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

// AllPermissions defines every permission in the system.
// Add new permissions here as features grow.
var AllPermissions = []models.Permission{
	{Name: "users:read",       Resource: "users",     Action: "read",    Description: "View user list and profiles"},
	{Name: "users:write",      Resource: "users",     Action: "write",   Description: "Create and update users"},
	{Name: "users:delete",     Resource: "users",     Action: "delete",  Description: "Delete users"},
	{Name: "roles:read",       Resource: "roles",     Action: "read",    Description: "View roles and permissions"},
	{Name: "roles:write",      Resource: "roles",     Action: "write",   Description: "Create and update roles"},
	{Name: "roles:delete",     Resource: "roles",     Action: "delete",  Description: "Delete roles"},
	{Name: "backtest:read",    Resource: "backtest",  Action: "read",    Description: "View backtest results"},
	{Name: "backtest:run",     Resource: "backtest",  Action: "run",     Description: "Submit new backtests"},
	{Name: "backtest:delete",  Resource: "backtest",  Action: "delete",  Description: "Delete backtest runs"},
	{Name: "portfolio:read",   Resource: "portfolio", Action: "read",    Description: "View portfolios and positions"},
	{Name: "portfolio:write",  Resource: "portfolio", Action: "write",   Description: "Manage portfolios and positions"},
	{Name: "alerts:read",      Resource: "alerts",    Action: "read",    Description: "View alerts"},
	{Name: "alerts:write",     Resource: "alerts",    Action: "write",   Description: "Create and manage alerts"},
	{Name: "monitor:read",     Resource: "monitor",   Action: "read",    Description: "View live monitors"},
	{Name: "monitor:write",    Resource: "monitor",   Action: "write",   Description: "Create and manage monitors"},
	{Name: "monitor:run",      Resource: "monitor",   Action: "run",     Description: "Start and stop monitors"},
	{Name: "news:read",        Resource: "news",      Action: "read",    Description: "View news articles"},
	{Name: "settings:read",    Resource: "settings",  Action: "read",    Description: "View settings"},
	{Name: "settings:write",   Resource: "settings",  Action: "write",   Description: "Change settings"},
	{Name: "audit:read",       Resource: "audit",     Action: "read",    Description: "View audit logs"},
	{Name: "system:read",      Resource: "system",    Action: "read",    Description: "View system stats and health"},
}

type roleSpec struct {
	name        string
	description string
	isSystem    bool
	perms       []string // permission names; "*" means all
}

var defaultRoles = []roleSpec{
	{
		name: "superadmin", description: "Full system access", isSystem: true,
		perms: []string{"*"},
	},
	{
		name: "admin", description: "User and role management", isSystem: true,
		perms: []string{
			"users:read", "users:write", "users:delete",
			"roles:read", "roles:write", "roles:delete",
			"backtest:read", "backtest:run", "backtest:delete",
			"portfolio:read", "portfolio:write",
			"alerts:read", "alerts:write",
			"monitor:read", "monitor:write", "monitor:run",
			"news:read", "settings:read", "settings:write",
			"audit:read", "system:read",
		},
	},
	{
		name: "trader", description: "Full trading access, no admin", isSystem: true,
		perms: []string{
			"backtest:read", "backtest:run", "backtest:delete",
			"portfolio:read", "portfolio:write",
			"alerts:read", "alerts:write",
			"monitor:read", "monitor:write", "monitor:run",
			"news:read", "settings:read",
		},
	},
	{
		name: "viewer", description: "Read-only access", isSystem: true,
		perms: []string{"backtest:read", "portfolio:read", "news:read"},
	},
}

// SeedRBAC upserts all permissions and default roles.
// Safe to call on every startup — idempotent.
func SeedRBAC(db *gorm.DB) error {
	// 1. Upsert all permissions
	permMap := make(map[string]models.Permission)
	for _, p := range AllPermissions {
		result := db.Where(models.Permission{Name: p.Name}).FirstOrCreate(&p)
		if result.Error != nil {
			return result.Error
		}
		permMap[p.Name] = p
	}
	log.Printf("[admin] seeded %d permissions", len(AllPermissions))

	// 2. Upsert default roles with permission associations
	for _, spec := range defaultRoles {
		var role models.Role
		db.Where(models.Role{Name: spec.name}).FirstOrCreate(&role, models.Role{
			Name:        spec.name,
			Description: spec.description,
			IsSystem:    spec.isSystem,
		})

		// Resolve permissions
		var permsToAssign []models.Permission
		if len(spec.perms) == 1 && spec.perms[0] == "*" {
			for _, p := range permMap {
				permsToAssign = append(permsToAssign, p)
			}
		} else {
			for _, name := range spec.perms {
				if p, ok := permMap[name]; ok {
					permsToAssign = append(permsToAssign, p)
				}
			}
		}
		if err := db.Model(&role).Association("Permissions").Replace(permsToAssign); err != nil {
			return err
		}
	}
	log.Printf("[admin] seeded %d default roles", len(defaultRoles))
	return nil
}
```

**Step 2: Call `SeedRBAC` in main.go** after AutoMigrate:

In `backend/cmd/server/main.go`, find the AutoMigrate block and add after it:
```go
if err := admin.SeedRBAC(db); err != nil {
    log.Fatalf("failed to seed RBAC: %v", err)
}
```

Add the import: `"github.com/trader-claude/backend/internal/admin"`

**Step 3: Verify seeding**
```bash
docker compose restart backend
docker compose logs backend | grep "\[admin\]"
# Expected:
# [admin] seeded 21 permissions
# [admin] seeded 4 default roles

make db-shell
SELECT name, is_system FROM roles;
SELECT name FROM permissions LIMIT 10;
SELECT COUNT(*) FROM role_permissions;
```

**Step 4: Commit**
```bash
git add backend/internal/admin/seed.go backend/cmd/server/main.go
git commit -m "feat(admin): add RBAC seeder — 21 permissions, 4 default roles, idempotent on startup"
```

---

## Task B3: Create RequirePermission middleware

**Files:**
- Create: `backend/internal/admin/middleware.go`

**Step 1: Create the file**

```go
package admin

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/models"
)

// RequirePermission returns a Fiber middleware that enforces a specific permission.
// Usage: admin.Group("/users", RequirePermission(db, "users:read"))
func RequirePermission(db *gorm.DB, permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role := auth.RoleFromCtx(c)          // e.g. "admin", "trader"
		if role == "" {
			return c.Status(401).JSON(fiber.Map{"error": "unauthenticated"})
		}

		// superadmin bypasses all permission checks
		if string(role) == string(models.UserRoleAdmin) {
			// Check if this user's DB role is actually superadmin
			var dbRole models.Role
			if db.Where("name = ?", "superadmin").First(&dbRole).Error == nil {
				// If user's role string matches superadmin name, bypass
				if strings.EqualFold(string(role), "superadmin") {
					return c.Next()
				}
			}
		}

		// Load role permissions from DB
		var dbRole models.Role
		if err := db.Preload("Permissions").Where("name = ?", string(role)).First(&dbRole).Error; err != nil {
			return c.Status(403).JSON(fiber.Map{"error": "role not found"})
		}

		// Check wildcard (superadmin)
		for _, p := range dbRole.Permissions {
			if p.Name == "*" {
				return c.Next()
			}
		}

		// Check specific permission
		for _, p := range dbRole.Permissions {
			if p.Name == permission {
				return c.Next()
			}
		}

		return c.Status(403).JSON(fiber.Map{"error": "forbidden: missing permission " + permission})
	}
}
```

**Step 2: Check what `auth.RoleFromCtx` is called in the codebase**
```bash
grep -r "RoleFromCtx\|Role.*Ctx\|locals.*role" backend/internal/auth/
```
Adapt the middleware to use whatever function exists for reading the role from Fiber context.

**Step 3: Build check**
```bash
docker compose exec backend go build ./...
```

**Step 4: Commit**
```bash
git add backend/internal/admin/middleware.go
git commit -m "feat(admin): add RequirePermission Fiber middleware for RBAC enforcement"
```

---

## Task B4: Extend admin handler — full user CRUD + audit logging

**Files:**
- Modify: `backend/internal/api/admin_handler.go`

**Step 1: Read the existing file first**
```bash
cat backend/internal/api/admin_handler.go
```

**Step 2: Add these new handler methods** to the existing `adminHandler` struct:

```go
func (h *adminHandler) getUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(user)
}

func (h *adminHandler) createUser(c *fiber.Ctx) error {
	var body struct {
		Email       string          `json:"email"`
		DisplayName string          `json:"display_name"`
		Password    string          `json:"password"`
		Role        models.UserRole `json:"role"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Email == "" || body.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "email and password required"})
	}
	// Use the existing auth service to hash password
	// If authSvc is not available here, inject it via constructor
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to hash password"})
	}
	user := models.User{
		Email:        body.Email,
		DisplayName:  body.DisplayName,
		PasswordHash: string(hash),
		Role:         body.Role,
		Active:       true,
	}
	if err := h.db.Create(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create user"})
	}
	h.writeAudit(c, "user.created", "user", fmt.Sprintf("%d", user.ID), nil)
	return c.Status(201).JSON(user)
}

func (h *adminHandler) updateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	var body struct {
		DisplayName string          `json:"display_name"`
		Role        models.UserRole `json:"role"`
		Active      *bool           `json:"active"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.DisplayName != "" { user.DisplayName = body.DisplayName }
	if body.Role != ""        { user.Role = body.Role }
	if body.Active != nil     { user.Active = *body.Active }
	if err := h.db.Save(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to update user"})
	}
	h.writeAudit(c, "user.updated", "user", id, fiber.Map{"role": user.Role, "active": user.Active})
	return c.JSON(user)
}

func (h *adminHandler) deleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	callerID := auth.UserIDFromCtx(c)
	if fmt.Sprintf("%d", callerID) == id {
		return c.Status(400).JSON(fiber.Map{"error": "cannot delete your own account"})
	}
	if err := h.db.Delete(&models.User{}, id).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to delete user"})
	}
	h.writeAudit(c, "user.deleted", "user", id, nil)
	return c.SendStatus(204)
}

// writeAudit inserts an AuditLog record. Non-blocking — errors are logged, not returned.
func (h *adminHandler) writeAudit(c *fiber.Ctx, action, resource, resourceID string, details interface{}) {
	userID := auth.UserIDFromCtx(c)
	var user models.User
	h.db.Select("email").First(&user, userID)
	entry := models.AuditLog{
		UserID:     userID,
		UserEmail:  user.Email,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  c.IP(),
	}
	if details != nil {
		b, _ := json.Marshal(details)
		m := models.JSON{}
		_ = json.Unmarshal(b, &m)
		entry.Details = m
	}
	h.db.Create(&entry)
}
```

Add missing imports at top of file:
```go
import (
    "encoding/json"
    "fmt"
    "golang.org/x/crypto/bcrypt"
    // existing imports...
)
```

**Step 3: Build check**
```bash
docker compose exec backend go build ./...
```

**Step 4: Commit**
```bash
git add backend/internal/api/admin_handler.go
git commit -m "feat(admin): extend admin handler with user CRUD and audit log writing"
```

---

## Task B5: Create roles handler

**Files:**
- Create: `backend/internal/api/roles_handler.go`

```go
package api

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type rolesHandler struct{ db *gorm.DB }

func newRolesHandler(db *gorm.DB) *rolesHandler { return &rolesHandler{db: db} }

func (h *rolesHandler) list(c *fiber.Ctx) error {
	var roles []models.Role
	if err := h.db.Preload("Permissions").Find(&roles).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch roles"})
	}
	return c.JSON(roles)
}

func (h *rolesHandler) get(c *fiber.Ctx) error {
	var role models.Role
	if err := h.db.Preload("Permissions").First(&role, c.Params("id")).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "role not found"})
	}
	return c.JSON(role)
}

func (h *rolesHandler) create(c *fiber.Ctx) error {
	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		PermIDs     []int64 `json:"permission_ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name required"})
	}
	role := models.Role{Name: body.Name, Description: body.Description}
	if err := h.db.Create(&role).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create role"})
	}
	if len(body.PermIDs) > 0 {
		var perms []models.Permission
		h.db.Find(&perms, body.PermIDs)
		h.db.Model(&role).Association("Permissions").Replace(perms)
	}
	h.db.Preload("Permissions").First(&role, role.ID)
	return c.Status(201).JSON(role)
}

func (h *rolesHandler) update(c *fiber.Ctx) error {
	var role models.Role
	if err := h.db.First(&role, c.Params("id")).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "role not found"})
	}
	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		PermIDs     []int64 `json:"permission_ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Name != "" { role.Name = body.Name }
	if body.Description != "" { role.Description = body.Description }
	h.db.Save(&role)
	if body.PermIDs != nil {
		var perms []models.Permission
		h.db.Find(&perms, body.PermIDs)
		h.db.Model(&role).Association("Permissions").Replace(perms)
	}
	h.db.Preload("Permissions").First(&role, role.ID)
	return c.JSON(role)
}

func (h *rolesHandler) delete(c *fiber.Ctx) error {
	var role models.Role
	if err := h.db.First(&role, c.Params("id")).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "role not found"})
	}
	if role.IsSystem {
		return c.Status(400).JSON(fiber.Map{"error": "system roles cannot be deleted"})
	}
	h.db.Model(&role).Association("Permissions").Clear()
	h.db.Delete(&role)
	return c.SendStatus(204)
}

func (h *rolesHandler) listPermissions(c *fiber.Ctx) error {
	var perms []models.Permission
	if err := h.db.Order("resource, action").Find(&perms).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch permissions"})
	}
	return c.JSON(perms)
}
```

**Step 2: Commit**
```bash
git add backend/internal/api/roles_handler.go
git commit -m "feat(admin): add roles handler — list, get, create, update, delete, list-permissions"
```

---

## Task B6: Create audit log handler

**Files:**
- Create: `backend/internal/api/audit_handler.go`

```go
package api

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type auditHandler struct{ db *gorm.DB }

func newAuditHandler(db *gorm.DB) *auditHandler { return &auditHandler{db: db} }

func (h *auditHandler) list(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	offset := (page - 1) * limit

	query := h.db.Model(&models.AuditLog{}).Order("created_at desc")

	// Filters
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if resource := c.Query("resource"); resource != "" {
		query = query.Where("resource = ?", resource)
	}
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if from := c.Query("from"); from != "" {
		query = query.Where("created_at >= ?", from)
	}
	if to := c.Query("to"); to != "" {
		query = query.Where("created_at <= ?", to)
	}

	var total int64
	query.Count(&total)

	var logs []models.AuditLog
	if err := query.Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch audit logs"})
	}
	return c.JSON(fiber.Map{
		"data":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
```

**Step 2: Commit**
```bash
git add backend/internal/api/audit_handler.go
git commit -m "feat(admin): add audit log handler with filtering and pagination"
```

---

## Task B7: Create system stats handler

**Files:**
- Create: `backend/internal/api/system_handler.go`

```go
package api

import (
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type systemHandler struct {
	db      *gorm.DB
	rdb     *redis.Client
	version string
	startAt time.Time
}

func newSystemHandler(db *gorm.DB, rdb *redis.Client, version string) *systemHandler {
	return &systemHandler{db: db, rdb: rdb, version: version, startAt: time.Now()}
}

func (h *systemHandler) stats(c *fiber.Ctx) error {
	var userCount, activeUsers, roleCount int64
	h.db.Model(&models.User{}).Count(&userCount)
	h.db.Model(&models.User{}).Where("active = true").Count(&activeUsers)
	h.db.Model(&models.Role{}).Count(&roleCount)

	var backtestCount, alertCount int64
	h.db.Model(&models.Backtest{}).Count(&backtestCount)
	h.db.Model(&models.Alert{}).Count(&alertCount)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return c.JSON(fiber.Map{
		"version":    h.version,
		"uptime_sec": int(time.Since(h.startAt).Seconds()),
		"go_version": runtime.Version(),
		"goroutines": runtime.NumGoroutine(),
		"memory_mb":  mem.Alloc / 1024 / 1024,
		"users": fiber.Map{
			"total":  userCount,
			"active": activeUsers,
		},
		"roles":     roleCount,
		"backtests": backtestCount,
		"alerts":    alertCount,
	})
}
```

**Step 2: Commit**
```bash
git add backend/internal/api/system_handler.go
git commit -m "feat(admin): add system stats handler — uptime, memory, entity counts"
```

---

## Task B8: Register all new admin routes

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Add handler inits** after `adminH := newAdminHandler(db)`:

```go
rolesH  := newRolesHandler(db)
auditH  := newAuditHandler(db)
sysH    := newSystemHandler(db, rdb, version)
```

**Step 2: Extend the `admin` group** with all new routes:

```go
// Existing
admin.Get("/users", adminH.listUsers)
admin.Patch("/users/:id/toggle", adminH.toggleUser)
admin.Patch("/users/:id/role", adminH.changeRole)

// New — full user CRUD
admin.Get("/users/:id", adminH.getUser)
admin.Post("/users", adminH.createUser)
admin.Put("/users/:id", adminH.updateUser)
admin.Delete("/users/:id", adminH.deleteUser)

// Roles + permissions
admin.Get("/roles", rolesH.list)
admin.Post("/roles", rolesH.create)
admin.Get("/roles/:id", rolesH.get)
admin.Put("/roles/:id", rolesH.update)
admin.Delete("/roles/:id", rolesH.delete)
admin.Get("/permissions", rolesH.listPermissions)

// Audit log
admin.Get("/audit-logs", auditH.list)

// System stats
admin.Get("/system/stats", sysH.stats)
```

**Step 3: Build + smoke test**
```bash
docker compose exec backend go build ./...
docker compose restart backend

# Test with admin token:
TOKEN="<your_admin_jwt>"
curl -s http://localhost:8080/api/v1/admin/roles -H "Authorization: Bearer $TOKEN" | jq .
curl -s http://localhost:8080/api/v1/admin/permissions -H "Authorization: Bearer $TOKEN" | jq '.[0:3]'
curl -s http://localhost:8080/api/v1/admin/system/stats -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: roles array with 4 roles + permissions list + system stats JSON.

**Step 4: Commit**
```bash
git add backend/internal/api/routes.go
git commit -m "feat(routes): register full admin CRUD routes — users, roles, permissions, audit, system"
```

---

## Backend Phase Completion Checklist

- [ ] `Permission`, `Role`, `AuditLog` models in `models.go`, tables auto-created
- [ ] `role_permissions` join table created by GORM
- [ ] `SeedRBAC()` runs on startup — 21 permissions + 4 default roles seeded
- [ ] `RequirePermission` middleware implemented
- [ ] Full user CRUD in admin handler (create, get, update, delete)
- [ ] `writeAudit()` called on all destructive admin actions
- [ ] Roles handler — list, create, update, delete, permission assignment
- [ ] Audit log handler — paginated, filterable
- [ ] System stats handler
- [ ] All routes registered under `/api/v1/admin/`
- [ ] `go build ./...` passes cleanly
- [ ] Smoke tests pass for roles, permissions, system stats
