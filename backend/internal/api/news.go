package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type newsHandler struct {
	db *gorm.DB
}

func newNewsHandler(db *gorm.DB) *newsHandler {
	return &newsHandler{db: db}
}

// GET /api/v1/news?limit=20&offset=0&symbol=BTCUSDT&from=RFC3339&to=RFC3339
func (h *newsHandler) listNews(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := c.QueryInt("offset", 0)
	symbol := c.Query("symbol")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	query := h.db.Model(&models.NewsItem{}).Order("published_at DESC")

	if symbol != "" {
		query = query.Where("JSON_CONTAINS(symbols, ?)", `"`+symbol+`"`)
	}
	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			query = query.Where("published_at >= ?", t)
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			query = query.Where("published_at <= ?", t)
		}
	}

	var total int64
	query.Count(&total)

	var items []models.NewsItem
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":      items,
		"total":     total,
		"page":      offset/limit + 1,
		"page_size": limit,
	})
}

// GET /api/v1/news/symbols/:symbol?limit=10
func (h *newsHandler) newsBySymbol(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	limit := c.QueryInt("limit", 10)
	if limit > 50 {
		limit = 50
	}

	var items []models.NewsItem
	if err := h.db.
		Where("JSON_CONTAINS(symbols, ?)", `"`+symbol+`"`).
		Order("published_at DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": items})
}
