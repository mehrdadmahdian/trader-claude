package indicator

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/trader-claude/backend/internal/registry"
)

// Handler exposes the indicator catalog and calculation endpoints.
type Handler struct{}

// NewHandler returns a new Handler.
func NewHandler() *Handler { return &Handler{} }

// ListIndicators handles GET /api/v1/indicators
func (h *Handler) ListIndicators(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"indicators": All()})
}

// calculateRequest is the body for POST /api/v1/indicators/calculate
type calculateRequest struct {
	IndicatorID string                 `json:"indicator_id"`
	Params      map[string]interface{} `json:"params"`
	Candles     []candleInput          `json:"candles"`
}

type candleInput struct {
	Timestamp int64   `json:"timestamp"` // Unix seconds
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// Calculate handles POST /api/v1/indicators/calculate
func (h *Handler) Calculate(c *fiber.Ctx) error {
	var req calculateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.IndicatorID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "indicator_id is required"})
	}
	if len(req.Candles) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "candles must not be empty"})
	}

	fn, err := Get(req.IndicatorID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Convert input candles to registry.Candle
	candles := make([]registry.Candle, len(req.Candles))
	for i, ci := range req.Candles {
		candles[i] = registry.Candle{
			Timestamp: time.Unix(ci.Timestamp, 0),
			Open:      ci.Open,
			High:      ci.High,
			Low:       ci.Low,
			Close:     ci.Close,
			Volume:    ci.Volume,
		}
	}

	result, err := fn(candles, req.Params)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	// Serialise NaN/Inf as null so JSON is valid
	serialised := make(map[string][]interface{}, len(result.Series))
	for name, series := range result.Series {
		out := make([]interface{}, len(series))
		for i, v := range series {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				out[i] = nil
			} else {
				out[i] = v
			}
		}
		serialised[name] = out
	}

	return c.JSON(fiber.Map{
		"timestamps": result.Timestamps,
		"series":     serialised,
	})
}
