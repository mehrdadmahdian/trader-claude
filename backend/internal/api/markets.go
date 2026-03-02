package api

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/registry"
)

var supportedTimeframes = []string{"1m", "5m", "15m", "30m", "1h", "4h", "1d", "1w"}

type marketsHandler struct {
	ds *adapter.DataService
}

func newMarketsHandler(ds *adapter.DataService) *marketsHandler {
	return &marketsHandler{ds: ds}
}

// listAdapters handles GET /api/v1/markets
func (h *marketsHandler) listAdapters(c *fiber.Ctx) error {
	adapters := registry.Adapters().All()
	result := make([]fiber.Map, 0, len(adapters))
	for _, a := range adapters {
		result = append(result, fiber.Map{
			"id":      a.Name(),
			"markets": a.Markets(),
			"healthy": a.IsHealthy(c.Context()),
		})
	}
	return c.JSON(fiber.Map{"data": result})
}

// listSymbols handles GET /api/v1/markets/:adapterID/symbols
func (h *marketsHandler) listSymbols(c *fiber.Ctx) error {
	adapterID := c.Params("adapterID")
	a, err := registry.Adapters().Get(adapterID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "adapter not found"})
	}

	market := c.Query("market", "")

	// If no market filter, use the first market the adapter supports
	if market == "" {
		markets := a.Markets()
		if len(markets) == 0 {
			return c.JSON(fiber.Map{"data": []interface{}{}})
		}
		market = markets[0]
	}

	symbols, err := a.FetchSymbols(c.Context(), market)
	if err != nil {
		log.Printf("listSymbols: FetchSymbols failed for adapter=%s market=%s: %v", adapterID, market, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to fetch symbols from upstream"})
	}

	result := make([]fiber.Map, 0, len(symbols))
	for _, s := range symbols {
		result = append(result, fiber.Map{
			"id":          s.ID,
			"market":      s.Market,
			"base_asset":  s.BaseAsset,
			"quote_asset": s.QuoteAsset,
			"description": s.Description,
			"active":      s.Active,
		})
	}
	return c.JSON(fiber.Map{"data": result})
}

// getCandles handles GET /api/v1/candles?adapter=&symbol=&timeframe=&from=&to=
func (h *marketsHandler) getCandles(c *fiber.Ctx) error {
	adapterID := c.Query("adapter", "")
	if adapterID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "adapter query param required"})
	}

	a, err := registry.Adapters().Get(adapterID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "adapter not found"})
	}

	symbol := c.Query("symbol", "")
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol query param required"})
	}

	timeframe := c.Query("timeframe", "1d")
	market := c.Query("market", "")
	if market == "" {
		markets := a.Markets()
		if len(markets) > 0 {
			market = markets[0]
		}
	}

	// Parse from/to; default to last 30 days
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30)
	to := now

	if fromStr := c.Query("from", ""); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from: use RFC3339"})
		}
		from = t
	}
	if toStr := c.Query("to", ""); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to: use RFC3339"})
		}
		to = t
	}

	candles, err := h.ds.GetCandles(c.Context(), a, symbol, market, timeframe, from, to)
	if err != nil {
		log.Printf("getCandles: GetCandles failed for adapter=%s symbol=%s timeframe=%s: %v", adapterID, symbol, timeframe, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	result := make([]fiber.Map, 0, len(candles))
	for _, candle := range candles {
		result = append(result, fiber.Map{
			"symbol":    candle.Symbol,
			"market":    candle.Market,
			"timeframe": candle.Timeframe,
			"timestamp": candle.Timestamp.Unix(),
			"open":      candle.Open,
			"high":      candle.High,
			"low":       candle.Low,
			"close":     candle.Close,
			"volume":    candle.Volume,
		})
	}
	return c.JSON(fiber.Map{"data": result})
}

// listTimeframes handles GET /api/v1/candles/timeframes
func (h *marketsHandler) listTimeframes(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"data": supportedTimeframes})
}
