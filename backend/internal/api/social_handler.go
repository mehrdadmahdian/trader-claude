package api

import (
	"encoding/base64"
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/notification"
	"github.com/trader-claude/backend/internal/social"
)

type socialHandler struct {
	db *gorm.DB
}

func newSocialHandler(db *gorm.DB) *socialHandler {
	return &socialHandler{db: db}
}

func (h *socialHandler) backtestCard(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("runId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid run id"})
	}
	theme := c.Query("theme", "dark")

	var bt models.Backtest
	if err := h.db.WithContext(c.Context()).First(&bt, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "backtest not found"})
	}

	// Extract metrics from Metrics JSON
	totalReturn := 0.0
	sharpeRatio := 0.0
	maxDrawdown := 0.0
	winRate := 0.0

	if bt.Metrics != nil {
		if tr, ok := bt.Metrics["total_return"].(float64); ok {
			totalReturn = tr
		}
		if sr, ok := bt.Metrics["sharpe_ratio"].(float64); ok {
			sharpeRatio = sr
		}
		if md, ok := bt.Metrics["max_drawdown"].(float64); ok {
			maxDrawdown = md
		}
		if wr, ok := bt.Metrics["win_rate"].(float64); ok {
			winRate = wr
		}
	}

	// Build equity curve from EquityCurve JSONArray
	curve := make([]float64, 0, len(bt.EquityCurve))
	for _, item := range bt.EquityCurve {
		if val, ok := item.(float64); ok {
			curve = append(curve, val)
		}
	}

	dateRange := ""
	if !bt.StartDate.IsZero() && !bt.EndDate.IsZero() {
		dateRange = bt.StartDate.Format("Jan 2, 2006") + " – " + bt.EndDate.Format("Jan 2, 2006")
	}

	png, err := social.GenerateBacktestCard(social.BacktestCardOpts{
		Theme:        theme,
		StrategyName: bt.StrategyName,
		Symbol:       bt.Symbol,
		Timeframe:    bt.Timeframe,
		DateRange:    dateRange,
		TotalReturn:  totalReturn,
		SharpeRatio:  sharpeRatio,
		MaxDrawdown:  maxDrawdown,
		WinRate:      winRate,
		EquityCurve:  curve,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "card generation failed"})
	}

	c.Set("Content-Type", "image/png")
	return c.Send(png)
}

func (h *socialHandler) signalCard(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("signalId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid signal id"})
	}
	theme := c.Query("theme", "dark")

	var sig models.MonitorSignal
	if err := h.db.WithContext(c.Context()).First(&sig, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "signal not found"})
	}

	// Load the monitor to get strategy name and symbol
	var monitor models.Monitor
	strategyName := ""
	symbol := ""
	if err := h.db.WithContext(c.Context()).First(&monitor, sig.MonitorID).Error; err == nil {
		strategyName = monitor.StrategyName
		symbol = monitor.Symbol
	}

	png, err := social.GenerateSignalCard(social.SignalCardOpts{
		Theme:        theme,
		Symbol:       symbol,
		Direction:    sig.Direction,
		Price:        sig.Price,
		StrategyName: strategyName,
		Strength:     sig.Strength,
		Timestamp:    sig.CreatedAt.Format("Jan 2, 2006 15:04 UTC"),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "card generation failed"})
	}

	c.Set("Content-Type", "image/png")
	return c.Send(png)
}

func (h *socialHandler) sendTelegram(c *fiber.Ctx) error {
	var body struct {
		ChatID      string `json:"chat_id"`
		Text        string `json:"text"`
		ImageBase64 string `json:"image_base64"`
		Caption     string `json:"caption"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	// Load Telegram settings from DB
	botToken, err := h.loadSettingString("notifications.telegram.bot_token")
	if err != nil || botToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "telegram not configured"})
	}

	chatID := body.ChatID
	if chatID == "" {
		chatID, _ = h.loadSettingString("notifications.telegram.chat_id")
	}
	if chatID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "chat_id required"})
	}

	sender := notification.NewTelegramSender(botToken)

	if body.ImageBase64 != "" {
		imgBytes, err := base64.StdEncoding.DecodeString(body.ImageBase64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid image_base64"})
		}
		if err := sender.SendPhoto(c.Context(), chatID, imgBytes, body.Caption); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	} else if body.Text != "" {
		if err := sender.SendText(c.Context(), chatID, body.Text); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "text or image_base64 required"})
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *socialHandler) loadSettingString(key string) (string, error) {
	var s models.Setting
	if err := h.db.Where("key = ?", key).First(&s).Error; err != nil {
		return "", err
	}
	var val string
	if s.Value != nil {
		// JSON type is a map[string]interface{}, so we need to marshal it back and unmarshal as string
		if b, err := json.Marshal(s.Value); err == nil {
			_ = json.Unmarshal(b, &val)
		}
	}
	return val, nil
}
