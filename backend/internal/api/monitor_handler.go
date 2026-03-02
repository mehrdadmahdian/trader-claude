package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/monitor"
	"github.com/trader-claude/backend/internal/registry"
	"github.com/trader-claude/backend/internal/validation"
)

type monitorHandler struct {
	db  *gorm.DB
	mgr *monitor.Manager
}

func newMonitorHandler(db *gorm.DB, mgr *monitor.Manager) *monitorHandler {
	return &monitorHandler{db: db, mgr: mgr}
}

// createMonitorReq is the validated request body for POST /api/v1/monitors.
type createMonitorReq struct {
	Name         string                 `json:"name" validate:"omitempty,max=100,safe_string"`
	AdapterID    string                 `json:"adapter_id" validate:"required,max=50"`
	Symbol       string                 `json:"symbol" validate:"required,symbol"`
	Market       string                 `json:"market" validate:"omitempty,market"`
	Timeframe    string                 `json:"timeframe" validate:"required,timeframe"`
	StrategyName string                 `json:"strategy_name" validate:"required,max=50"`
	Params       map[string]interface{} `json:"params"`
	NotifyInApp  *bool                  `json:"notify_in_app"`
}

// POST /api/v1/monitors
func (h *monitorHandler) createMonitor(c *fiber.Ctx) error {
	var body createMonitorReq
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validation.Validate(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request parameters"})
	}
	if body.Symbol == "" || body.AdapterID == "" || body.StrategyName == "" || body.Timeframe == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol, adapter_id, strategy_name and timeframe are required"})
	}
	if !registry.Strategies().Exists(body.StrategyName) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unknown strategy: " + body.StrategyName})
	}

	notifyInApp := true
	if body.NotifyInApp != nil {
		notifyInApp = *body.NotifyInApp
	}

	name := body.Name
	if name == "" {
		name = body.StrategyName + " " + body.Symbol + " " + body.Timeframe
	}

	mon := models.Monitor{
		Name:         name,
		AdapterID:    body.AdapterID,
		Symbol:       body.Symbol,
		Market:       body.Market,
		Timeframe:    body.Timeframe,
		StrategyName: body.StrategyName,
		Params:       models.JSON(body.Params),
		Status:       models.MonitorStatusActive,
		NotifyInApp:  notifyInApp,
		UserID:       auth.GetUserID(c),
	}

	if err := h.db.Create(&mon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	h.mgr.Add(mon.ID, mon.Timeframe)

	return c.Status(fiber.StatusCreated).JSON(mon)
}

// GET /api/v1/monitors
func (h *monitorHandler) listMonitors(c *fiber.Ctx) error {
	var monitors []models.Monitor
	if err := h.db.Where("user_id = ?", auth.GetUserID(c)).Order("created_at DESC").Find(&monitors).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": monitors})
}

// GET /api/v1/monitors/:id
func (h *monitorHandler) getMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var mon models.Monitor
	if err := h.db.Where("id = ? AND user_id = ?", id, auth.GetUserID(c)).First(&mon).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mon)
}

// PUT /api/v1/monitors/:id
func (h *monitorHandler) updateMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	userID := auth.GetUserID(c)
	var mon models.Monitor
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&mon).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	var body struct {
		Name        string `json:"name"`
		NotifyInApp *bool  `json:"notify_in_app"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	updates := map[string]interface{}{}
	if body.Name != "" {
		updates["name"] = body.Name
	}
	if body.NotifyInApp != nil {
		updates["notify_in_app"] = *body.NotifyInApp
	}
	if len(updates) > 0 {
		if err := h.db.Model(&mon).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&mon).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	return c.JSON(mon)
}

// DELETE /api/v1/monitors/:id
func (h *monitorHandler) deleteMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.db.Where("user_id = ?", auth.GetUserID(c)).Delete(&models.Monitor{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.mgr.Remove(int64(id))
	return c.SendStatus(fiber.StatusNoContent)
}

// PATCH /api/v1/monitors/:id/toggle
// Flips status: active → paused, paused → active.
func (h *monitorHandler) toggleMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var mon models.Monitor
	if err := h.db.Where("id = ? AND user_id = ?", id, auth.GetUserID(c)).First(&mon).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	switch mon.Status {
	case models.MonitorStatusActive:
		if err := h.db.Model(&mon).Update("status", models.MonitorStatusPaused).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		h.mgr.Pause(int64(id))
	case models.MonitorStatusPaused:
		if err := h.db.Model(&mon).Update("status", models.MonitorStatusActive).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		h.mgr.Resume(int64(id), mon.Timeframe)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor is stopped"})
	}

	if err := h.db.First(&mon, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mon)
}

// GET /api/v1/monitors/:id/signals?limit=20&offset=0
func (h *monitorHandler) listSignals(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := c.QueryInt("offset", 0)

	// Verify monitor exists and belongs to user
	var mon models.Monitor
	if err := h.db.Select("id").Where("id = ? AND user_id = ?", id, auth.GetUserID(c)).First(&mon).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var total int64
	if err := h.db.Model(&models.MonitorSignal{}).Where("monitor_id = ?", id).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var signals []models.MonitorSignal
	if err := h.db.Where("monitor_id = ?", id).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&signals).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":   signals,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
