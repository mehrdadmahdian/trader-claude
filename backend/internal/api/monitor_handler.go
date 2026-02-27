package api

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/monitor"
	"github.com/trader-claude/backend/internal/registry"
)

type monitorHandler struct {
	db  *gorm.DB
	mgr *monitor.Manager
}

func newMonitorHandler(db *gorm.DB, mgr *monitor.Manager) *monitorHandler {
	return &monitorHandler{db: db, mgr: mgr}
}

// POST /api/v1/monitors
func (h *monitorHandler) createMonitor(c *fiber.Ctx) error {
	var body struct {
		Name         string                 `json:"name"`
		AdapterID    string                 `json:"adapter_id"`
		Symbol       string                 `json:"symbol"`
		Market       string                 `json:"market"`
		Timeframe    string                 `json:"timeframe"`
		StrategyName string                 `json:"strategy_name"`
		Params       map[string]interface{} `json:"params"`
		NotifyInApp  *bool                  `json:"notify_in_app"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
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
	if err := h.db.Order("created_at DESC").Find(&monitors).Error; err != nil {
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
	if err := h.db.First(&mon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(mon)
}

// PUT /api/v1/monitors/:id
func (h *monitorHandler) updateMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var mon models.Monitor
	if err := h.db.First(&mon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
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
	}
	return c.JSON(mon)
}

// DELETE /api/v1/monitors/:id
func (h *monitorHandler) deleteMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	h.mgr.Remove(int64(id))
	if err := h.db.Delete(&models.Monitor{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
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
	if err := h.db.First(&mon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
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

	h.db.First(&mon, id) // reload
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

	var total int64
	h.db.Model(&models.MonitorSignal{}).Where("monitor_id = ?", id).Count(&total)

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
