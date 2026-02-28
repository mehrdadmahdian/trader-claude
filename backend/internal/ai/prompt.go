package ai

import (
	"fmt"
	"strings"
)

// PageContext describes the user's current view for system prompt injection.
type PageContext struct {
	Page       string                 `json:"page"`
	Symbol     string                 `json:"symbol,omitempty"`
	Timeframe  string                 `json:"timeframe,omitempty"`
	Indicators []string               `json:"indicators,omitempty"`
	Metrics    map[string]interface{} `json:"metrics,omitempty"`
	Positions  []PositionSummary      `json:"positions,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// PositionSummary is a lightweight position description for the AI context.
type PositionSummary struct {
	Symbol string  `json:"symbol"`
	PnLPct float64 `json:"pnl_pct"`
}

const baseSystemPrompt = `You are an AI trading assistant for the trader-claude platform. You help users understand their market data, backtest results, portfolio performance, and trading strategies. Be concise, accurate, and actionable. Always end your response with exactly 3 suggested follow-up questions formatted as:

` + "```suggestions" + `
["question 1", "question 2", "question 3"]
` + "```"

// BuildSystemPrompt creates a context-specific system prompt from the user's current page state.
func BuildSystemPrompt(ctx PageContext) string {
	var sb strings.Builder
	sb.WriteString(baseSystemPrompt)
	sb.WriteString("\n\n")

	switch ctx.Page {
	case "chart":
		sb.WriteString(fmt.Sprintf("The user is viewing a %s chart for %s.", ctx.Timeframe, ctx.Symbol))
		if len(ctx.Indicators) > 0 {
			sb.WriteString(fmt.Sprintf(" Active indicators: %s.", strings.Join(ctx.Indicators, ", ")))
		}
	case "backtest":
		sb.WriteString("The user is viewing backtest results.")
		if len(ctx.Metrics) > 0 {
			sb.WriteString(" Metrics:")
			for k, v := range ctx.Metrics {
				sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
			}
			sb.WriteString(".")
		}
	case "portfolio":
		sb.WriteString("The user is viewing their portfolio.")
		if len(ctx.Positions) > 0 {
			sb.WriteString(fmt.Sprintf(" They have %d positions.", len(ctx.Positions)))
			for _, p := range ctx.Positions {
				sb.WriteString(fmt.Sprintf(" %s: %+.1f%%", p.Symbol, p.PnLPct))
			}
		}
	case "monitor":
		sb.WriteString("The user is viewing live market monitors.")
	case "alerts":
		sb.WriteString("The user is viewing price alerts.")
	case "news":
		sb.WriteString(fmt.Sprintf("The user is reading news for %s.", ctx.Symbol))
	default:
		sb.WriteString("The user is on the general dashboard.")
	}

	return sb.String()
}
