package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/models"
)

type workspaceHandler struct{ db *gorm.DB }

func newWorkspaceHandler(db *gorm.DB) *workspaceHandler {
	return &workspaceHandler{db: db}
}

func (h *workspaceHandler) list(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	var workspaces []models.Workspace
	if err := h.db.Where("user_id = ?", userID).Order("created_at asc").Find(&workspaces).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch workspaces"})
	}
	return c.JSON(workspaces)
}

func (h *workspaceHandler) get(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	id := c.Params("id")
	var ws models.Workspace
	err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch workspace"})
	}
	return c.JSON(ws)
}

func (h *workspaceHandler) create(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	var body struct {
		Name        string      `json:"name"`
		Layout      models.JSON `json:"layout"`
		PanelStates models.JSON `json:"panel_states"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	ws := models.Workspace{
		UserID:      userID,
		Name:        body.Name,
		Layout:      body.Layout,
		PanelStates: body.PanelStates,
	}
	if err := h.db.Create(&ws).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create workspace"})
	}
	return c.Status(201).JSON(ws)
}

func (h *workspaceHandler) update(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	id := c.Params("id")
	var ws models.Workspace
	err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch workspace"})
	}
	var body struct {
		Name        string      `json:"name"`
		Layout      models.JSON `json:"layout"`
		PanelStates models.JSON `json:"panel_states"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Name != "" {
		ws.Name = body.Name
	}
	if body.Layout != nil {
		ws.Layout = body.Layout
	}
	if body.PanelStates != nil {
		ws.PanelStates = body.PanelStates
	}
	if err := h.db.Save(&ws).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to update workspace"})
	}
	return c.JSON(ws)
}

func (h *workspaceHandler) delete(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	id := c.Params("id")
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Workspace{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to delete workspace"})
	}
	return c.SendStatus(204)
}
