package api

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"

	"github.com/trader-claude/backend/internal/portfolio"
)

// PortfolioUpdateMsg is the message pushed to WS clients every 5s
type PortfolioUpdateMsg struct {
	Type        string                `json:"type"`
	PortfolioID int64                 `json:"portfolio_id"`
	TotalValue  float64               `json:"total_value"`
	TotalPnL    float64               `json:"total_pnl"`
	TotalPnLPct float64               `json:"total_pnl_pct"`
	Positions   []PositionUpdateEntry `json:"positions"`
}

// PositionUpdateEntry is a single position's live data within a PortfolioUpdateMsg
type PositionUpdateEntry struct {
	ID               int64   `json:"id"`
	Symbol           string  `json:"symbol"`
	CurrentPrice     float64 `json:"current_price"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
}

func portfolioLiveWS(svc *portfolio.Service, userID int64) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		idStr := conn.Params("id")
		portfolioID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid portfolio id"))
			return
		}

		// Send immediately on connect
		if err := sendPortfolioUpdate(conn, svc, portfolioID, userID); err != nil {
			return
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := sendPortfolioUpdate(conn, svc, portfolioID, userID); err != nil {
					return
				}
			}
		}
	}
}

func sendPortfolioUpdate(conn *websocket.Conn, svc *portfolio.Service, portfolioID, userID int64) error {
	ctx := context.Background()

	if err := svc.RecalculatePortfolio(ctx, portfolioID); err != nil {
		// Log but don't disconnect — price service may be temporarily unavailable
		log.Printf("portfolio WS: recalculate error for %d: %v", portfolioID, err)
	}

	_, positions, err := svc.GetPortfolioWithPositions(ctx, portfolioID, userID)
	if err != nil {
		return err
	}

	sum, err := svc.GetSummary(ctx, portfolioID, userID)
	if err != nil {
		return err
	}

	entries := make([]PositionUpdateEntry, 0, len(positions))
	for _, pos := range positions {
		entries = append(entries, PositionUpdateEntry{
			ID:               pos.ID,
			Symbol:           pos.Symbol,
			CurrentPrice:     pos.CurrentPrice,
			UnrealizedPnL:    pos.UnrealizedPnL,
			UnrealizedPnLPct: pos.UnrealizedPnLPct,
		})
	}

	msg := PortfolioUpdateMsg{
		Type:        "portfolio_update",
		PortfolioID: portfolioID,
		TotalValue:  sum.TotalValue,
		TotalPnL:    sum.TotalPnL,
		TotalPnLPct: sum.TotalPnLPct,
		Positions:   entries,
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, b)
}
