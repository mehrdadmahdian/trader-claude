package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/portfolio"
)

type portfolioHandler struct {
	svc *portfolio.Service
}

func newPortfolioHandler(svc *portfolio.Service) *portfolioHandler {
	return &portfolioHandler{svc: svc}
}

func (h *portfolioHandler) registerRoutes(v1 fiber.Router) {
	v1.Post("/portfolios", h.create)
	v1.Get("/portfolios", h.list)
	v1.Get("/portfolios/:id", h.get)
	v1.Put("/portfolios/:id", h.update)
	v1.Delete("/portfolios/:id", h.delete)
	v1.Get("/portfolios/:id/summary", h.summary)

	v1.Post("/portfolios/:id/positions", h.addPosition)
	v1.Put("/portfolios/:id/positions/:posId", h.updatePosition)
	v1.Delete("/portfolios/:id/positions/:posId", h.deletePosition)

	v1.Post("/portfolios/:id/transactions", h.addTransaction)
	v1.Get("/portfolios/:id/transactions", h.listTransactions)

	v1.Get("/portfolios/:id/equity-curve", h.equityCurve)
}

func parsePortfolioID(c *fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid portfolio id")
	}
	return id, nil
}

func parsePosID(c *fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("posId"), 10, 64)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid position id")
	}
	return id, nil
}

func (h *portfolioHandler) create(c *fiber.Ctx) error {
	var req portfolio.CreatePortfolioReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	p, err := h.svc.CreatePortfolio(c.Context(), auth.GetUserID(c), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": p})
}

func (h *portfolioHandler) list(c *fiber.Ctx) error {
	portfolios, err := h.svc.ListPortfolios(c.Context(), auth.GetUserID(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": portfolios})
}

func (h *portfolioHandler) get(c *fiber.Ctx) error {
	id, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	p, positions, err := h.svc.GetPortfolioWithPositions(c.Context(), id, auth.GetUserID(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "portfolio not found"})
	}
	return c.JSON(fiber.Map{"data": p, "positions": positions})
}

func (h *portfolioHandler) update(c *fiber.Ctx) error {
	id, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	var req portfolio.UpdatePortfolioReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	p, err := h.svc.UpdatePortfolio(c.Context(), id, auth.GetUserID(c), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": p})
}

func (h *portfolioHandler) delete(c *fiber.Ctx) error {
	id, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeletePortfolio(c.Context(), id, auth.GetUserID(c)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *portfolioHandler) summary(c *fiber.Ctx) error {
	id, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	sum, err := h.svc.GetSummary(c.Context(), id, auth.GetUserID(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "portfolio not found"})
	}
	return c.JSON(fiber.Map{"data": sum})
}

func (h *portfolioHandler) addPosition(c *fiber.Ctx) error {
	id, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	var req portfolio.AddPositionReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	pos, err := h.svc.AddPosition(c.Context(), id, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": pos})
}

func (h *portfolioHandler) updatePosition(c *fiber.Ctx) error {
	portfolioID, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	posID, err := parsePosID(c)
	if err != nil {
		return err
	}
	// Verify position belongs to this portfolio
	existing, err := h.svc.GetPosition(c.Context(), posID)
	if err != nil || existing.PortfolioID != portfolioID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "position not found"})
	}
	var req portfolio.UpdatePositionReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	pos, err := h.svc.UpdatePosition(c.Context(), posID, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": pos})
}

func (h *portfolioHandler) deletePosition(c *fiber.Ctx) error {
	portfolioID, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	posID, err := parsePosID(c)
	if err != nil {
		return err
	}
	// Verify position belongs to this portfolio
	existing, err := h.svc.GetPosition(c.Context(), posID)
	if err != nil || existing.PortfolioID != portfolioID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "position not found"})
	}
	if err := h.svc.DeletePosition(c.Context(), posID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *portfolioHandler) addTransaction(c *fiber.Ctx) error {
	id, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	var req portfolio.AddTransactionReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	tx, err := h.svc.AddTransaction(c.Context(), id, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": tx})
}

func (h *portfolioHandler) listTransactions(c *fiber.Ctx) error {
	id, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	txs, total, err := h.svc.ListTransactions(c.Context(), id, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": txs, "total": total, "page": page, "page_size": limit})
}

func (h *portfolioHandler) equityCurve(c *fiber.Ctx) error {
	id, err := parsePortfolioID(c)
	if err != nil {
		return err
	}
	points, err := h.svc.GetEquityCurve(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"points": points})
}
