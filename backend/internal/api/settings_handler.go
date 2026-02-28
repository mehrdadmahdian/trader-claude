package api

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/notification"
)

type settingsHandler struct {
	db *gorm.DB
}

func newSettingsHandler(db *gorm.DB) *settingsHandler {
	return &settingsHandler{db: db}
}

type notificationSettings struct {
	Telegram struct {
		BotToken string `json:"bot_token"`
		ChatID   string `json:"chat_id"`
		Enabled  bool   `json:"enabled"`
	} `json:"telegram"`
	Webhook struct {
		URL     string `json:"url"`
		Secret  string `json:"secret"`
		Enabled bool   `json:"enabled"`
	} `json:"webhook"`
}

func (h *settingsHandler) getNotificationSettings(c *fiber.Ctx) error {
	keys := []string{
		"notifications.telegram.bot_token",
		"notifications.telegram.chat_id",
		"notifications.telegram.enabled",
		"notifications.webhook.url",
		"notifications.webhook.secret",
		"notifications.webhook.enabled",
	}

	var settings []models.Setting
	if err := h.db.WithContext(c.Context()).Where("key IN ?", keys).Find(&settings).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load settings"})
	}

	// Build map
	vals := make(map[string]string)
	for _, s := range settings {
		var v string
		if s.Value != nil {
			if b, err := json.Marshal(s.Value); err == nil {
				_ = json.Unmarshal(b, &v)
			}
		}
		vals[s.Key] = v
	}

	var ns notificationSettings
	ns.Telegram.BotToken = vals["notifications.telegram.bot_token"]
	ns.Telegram.ChatID = vals["notifications.telegram.chat_id"]
	ns.Telegram.Enabled = vals["notifications.telegram.enabled"] == "true"
	ns.Webhook.URL = vals["notifications.webhook.url"]
	ns.Webhook.Secret = vals["notifications.webhook.secret"]
	ns.Webhook.Enabled = vals["notifications.webhook.enabled"] == "true"

	return c.JSON(ns)
}

func (h *settingsHandler) saveNotificationSettings(c *fiber.Ctx) error {
	var ns notificationSettings
	if err := c.BodyParser(&ns); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	toSave := map[string]interface{}{
		"notifications.telegram.bot_token": ns.Telegram.BotToken,
		"notifications.telegram.chat_id":   ns.Telegram.ChatID,
		"notifications.webhook.url":        ns.Webhook.URL,
		"notifications.webhook.secret":     ns.Webhook.Secret,
	}
	if ns.Telegram.Enabled {
		toSave["notifications.telegram.enabled"] = "true"
	} else {
		toSave["notifications.telegram.enabled"] = "false"
	}
	if ns.Webhook.Enabled {
		toSave["notifications.webhook.enabled"] = "true"
	} else {
		toSave["notifications.webhook.enabled"] = "false"
	}

	for key, val := range toSave {
		// Marshal the value as a JSON string, then store as a JSON object
		b, err := json.Marshal(val)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encode setting"})
		}
		var jsonVal models.JSON
		if err := json.Unmarshal(b, &jsonVal); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encode setting"})
		}

		setting := models.Setting{
			UserID: 1,
			Key:    key,
			Value:  jsonVal,
		}
		result := h.db.WithContext(c.Context()).
			Where(models.Setting{UserID: 1, Key: key}).
			Assign(models.Setting{Value: jsonVal}).
			FirstOrCreate(&setting)
		if result.Error != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save setting"})
		}
	}

	return c.JSON(fiber.Map{"saved": true})
}

func (h *settingsHandler) testNotificationSettings(c *fiber.Ctx) error {
	result := fiber.Map{}

	// Test Telegram
	var tgBot, tgChat, tgEnabled models.Setting
	h.db.WithContext(c.Context()).Where("key = ?", "notifications.telegram.bot_token").First(&tgBot)
	h.db.WithContext(c.Context()).Where("key = ?", "notifications.telegram.chat_id").First(&tgChat)
	h.db.WithContext(c.Context()).Where("key = ?", "notifications.telegram.enabled").First(&tgEnabled)

	var botToken, chatID, enabled string
	if tgBot.Value != nil {
		if b, err := json.Marshal(tgBot.Value); err == nil {
			_ = json.Unmarshal(b, &botToken)
		}
	}
	if tgChat.Value != nil {
		if b, err := json.Marshal(tgChat.Value); err == nil {
			_ = json.Unmarshal(b, &chatID)
		}
	}
	if tgEnabled.Value != nil {
		if b, err := json.Marshal(tgEnabled.Value); err == nil {
			_ = json.Unmarshal(b, &enabled)
		}
	}

	if enabled == "true" && botToken != "" {
		sender := notification.NewTelegramSender(botToken)
		name, err := sender.TestConnection(c.Context())
		if err != nil {
			result["telegram"] = fiber.Map{"ok": false, "error": err.Error()}
		} else {
			result["telegram"] = fiber.Map{"ok": true, "bot_name": name}
		}
	} else {
		result["telegram"] = fiber.Map{"ok": false, "error": "telegram not configured or disabled"}
	}

	return c.JSON(result)
}
