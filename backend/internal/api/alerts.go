package api

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/price"
	"github.com/trader-claude/backend/internal/validation"
)

type alertHandler struct {
	db       *gorm.DB
	priceSvc *price.Service
}

func newAlertHandler(db *gorm.DB, priceSvc *price.Service) *alertHandler {
	return &alertHandler{db: db, priceSvc: priceSvc}
}

type createAlertReq struct {
	Name             string                `json:"name" validate:"required,max=100,safe_string"`
	AdapterID        string                `json:"adapter_id" validate:"required,max=50"`
	Symbol           string                `json:"symbol" validate:"required,symbol"`
	Market           string                `json:"market" validate:"omitempty,market"`
	Condition        models.AlertCondition `json:"condition" validate:"required"`
	Threshold        float64               `json:"threshold" validate:"gte=0"`
	RecurringEnabled bool                  `json:"recurring_enabled"`
	CooldownMinutes  int                   `json:"cooldown_minutes" validate:"gte=0,lte=10080"`
}

// POST /api/v1/alerts
func (h *alertHandler) createAlert(c *fiber.Ctx) error {
	var req createAlertReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := validation.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request parameters"})
	}
	if req.Name == "" || req.Symbol == "" || req.AdapterID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name, symbol, and adapter_id are required",
		})
	}
	if req.CooldownMinutes == 0 {
		req.CooldownMinutes = 60
	}

	a := models.Alert{
		Name:             req.Name,
		AdapterID:        req.AdapterID,
		Symbol:           req.Symbol,
		Market:           req.Market,
		Condition:        req.Condition,
		Threshold:        req.Threshold,
		Status:           models.AlertStatusActive,
		RecurringEnabled: req.RecurringEnabled,
		CooldownMinutes:  req.CooldownMinutes,
		UserID:           auth.GetUserID(c),
	}

	// For price_change_pct alerts, store the current price as the base reference.
	if req.Condition == models.AlertConditionPriceChange {
		if basePrice, err := h.priceSvc.GetPrice(c.Context(), req.AdapterID, req.Symbol); err == nil {
			a.BasePrice = basePrice
		}
	}

	if err := h.db.Create(&a).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": a})
}

// GET /api/v1/alerts
func (h *alertHandler) listAlerts(c *fiber.Ctx) error {
	var alerts []models.Alert
	if err := h.db.Where("user_id = ?", auth.GetUserID(c)).Order("created_at DESC").Find(&alerts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": alerts})
}

// DELETE /api/v1/alerts/:id
func (h *alertHandler) deleteAlert(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.db.Where("user_id = ?", auth.GetUserID(c)).Delete(&models.Alert{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PATCH /api/v1/alerts/:id/toggle — toggles between active and disabled
func (h *alertHandler) toggleAlert(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var a models.Alert
	if err := h.db.Where("id = ? AND user_id = ?", id, auth.GetUserID(c)).First(&a).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "alert not found"})
	}
	newStatus := models.AlertStatusDisabled
	if a.Status == models.AlertStatusDisabled {
		newStatus = models.AlertStatusActive
	}
	if err := h.db.Model(&a).Update("status", newStatus).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update alert"})
	}
	a.Status = newStatus
	return c.JSON(fiber.Map{"data": a})
}
