package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/ai"
	"github.com/trader-claude/backend/internal/models"
)

type aiHandler struct {
	db *gorm.DB
}

func newAIHandler(db *gorm.DB) *aiHandler {
	return &aiHandler{db: db}
}

type aiChatRequest struct {
	Messages    []ai.ChatMessage `json:"messages"`
	PageContext ai.PageContext   `json:"page_context"`
	Provider    string           `json:"provider"`
}

func (h *aiHandler) chat(c *fiber.Ctx) error {
	var req aiChatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if len(req.Messages) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "messages required"})
	}

	// Load AI settings
	apiKey, _ := h.loadSetting("ai.api_key")
	model, _ := h.loadSetting("ai.model")
	ollamaURL, _ := h.loadSetting("ai.ollama_url")
	providerName, _ := h.loadSetting("ai.provider")
	if req.Provider != "" {
		providerName = req.Provider
	}
	if providerName == "" {
		providerName = "openai"
	}

	// Build system prompt
	systemPrompt := ai.BuildSystemPrompt(req.PageContext)
	messages := append([]ai.ChatMessage{
		{Role: ai.RoleSystem, Content: systemPrompt},
	}, req.Messages...)

	// Create provider
	var provider ai.AIProvider
	switch providerName {
	case "ollama":
		provider = ai.NewOllamaProvider(ollamaURL, model)
	default:
		if apiKey == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "OpenAI API key not configured"})
		}
		provider = ai.NewOpenAIProvider(apiKey, model)
	}

	resp, err := provider.Chat(c.Context(), messages)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "AI request failed"})
	}

	return c.JSON(resp)
}

type aiSettings struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	OllamaURL string `json:"ollama_url"`
	HasAPIKey bool   `json:"has_api_key"`
}

func (h *aiHandler) getAISettings(c *fiber.Ctx) error {
	provider, _ := h.loadSetting("ai.provider")
	model, _ := h.loadSetting("ai.model")
	ollamaURL, _ := h.loadSetting("ai.ollama_url")
	apiKey, _ := h.loadSetting("ai.api_key")

	if provider == "" {
		provider = "openai"
	}

	return c.JSON(aiSettings{
		Provider:  provider,
		Model:     model,
		OllamaURL: ollamaURL,
		HasAPIKey: apiKey != "",
	})
}

func (h *aiHandler) saveAISettings(c *fiber.Ctx) error {
	var body struct {
		Provider  string `json:"provider"`
		Model     string `json:"model"`
		OllamaURL string `json:"ollama_url"`
		APIKey    string `json:"api_key"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	toSave := map[string]string{
		"ai.provider":   body.Provider,
		"ai.model":      body.Model,
		"ai.ollama_url": body.OllamaURL,
	}
	// Only overwrite api_key if a new one was provided
	if body.APIKey != "" {
		toSave["ai.api_key"] = body.APIKey
	}

	for key, val := range toSave {
		if err := h.saveSetting(c, key, val); err != nil {
			return err
		}
	}

	return c.JSON(fiber.Map{"saved": true})
}

func (h *aiHandler) testAIConnection(c *fiber.Ctx) error {
	providerName, _ := h.loadSetting("ai.provider")
	apiKey, _ := h.loadSetting("ai.api_key")
	model, _ := h.loadSetting("ai.model")
	ollamaURL, _ := h.loadSetting("ai.ollama_url")

	if providerName == "" {
		providerName = "openai"
	}

	var provider ai.AIProvider
	switch providerName {
	case "ollama":
		provider = ai.NewOllamaProvider(ollamaURL, model)
	default:
		if apiKey == "" {
			return c.JSON(fiber.Map{"ok": false, "error": "API key not configured"})
		}
		provider = ai.NewOpenAIProvider(apiKey, model)
	}

	if err := provider.TestConnection(c.Context()); err != nil {
		return c.JSON(fiber.Map{"ok": false, "error": "connection test failed"})
	}

	return c.JSON(fiber.Map{"ok": true})
}

// loadSetting retrieves a string value from the settings table.
func (h *aiHandler) loadSetting(key string) (string, error) {
	var s models.Setting
	if err := h.db.Where("key = ?", key).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if s.Value == nil {
		return "", nil
	}
	if v, ok := s.Value["value"].(string); ok {
		return v, nil
	}
	return "", nil
}

// saveSetting upserts a string value into the settings table.
func (h *aiHandler) saveSetting(c *fiber.Ctx, key, val string) error {
	setting := models.Setting{
		UserID: 1,
		Key:    key,
		Value:  models.JSON{"value": val},
	}
	result := h.db.WithContext(c.Context()).
		Where(models.Setting{UserID: 1, Key: key}).
		Assign(models.Setting{Value: models.JSON{"value": val}}).
		FirstOrCreate(&setting)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save setting"})
	}
	return nil
}
