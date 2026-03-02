package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/models"
)

type adminHandler struct {
	db *gorm.DB
}

func newAdminHandler(db *gorm.DB) *adminHandler {
	return &adminHandler{db: db}
}

// listUsers returns all users (paginated, admin only)
func (h *adminHandler) listUsers(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	var users []models.User
	var total int64

	if err := h.db.WithContext(c.Context()).Model(&models.User{}).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to count users"})
	}

	if err := h.db.WithContext(c.Context()).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list users"})
	}

	return c.JSON(fiber.Map{"data": users, "total": total, "page": page, "limit": limit})
}

// toggleUser toggles a user's active status (admin only, cannot toggle self)
func (h *adminHandler) toggleUser(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
	}

	if id == auth.GetUserID(c) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot modify your own account"})
	}

	var user models.User
	if err := h.db.WithContext(c.Context()).First(&user, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	user.Active = !user.Active
	if err := h.db.WithContext(c.Context()).Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update user"})
	}

	return c.JSON(user)
}

// changeRole updates a user's role (admin only)
func (h *adminHandler) changeRole(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
	}

	if id == auth.GetUserID(c) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot change your own role"})
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Role != string(models.UserRoleAdmin) && req.Role != string(models.UserRoleUser) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role must be 'admin' or 'user'"})
	}

	var user models.User
	if err := h.db.WithContext(c.Context()).First(&user, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	user.Role = models.UserRole(req.Role)
	if err := h.db.WithContext(c.Context()).Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update role"})
	}

	return c.JSON(user)
}
