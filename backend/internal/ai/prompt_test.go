package ai

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_Chart(t *testing.T) {
	prompt := BuildSystemPrompt(PageContext{
		Page:       "chart",
		Symbol:     "BTCUSDT",
		Timeframe:  "1h",
		Indicators: []string{"RSI(14)", "EMA(21)"},
	})
	if !strings.Contains(prompt, "BTCUSDT") {
		t.Error("expected symbol in prompt")
	}
	if !strings.Contains(prompt, "RSI(14)") {
		t.Error("expected indicator in prompt")
	}
}

func TestBuildSystemPrompt_Backtest(t *testing.T) {
	prompt := BuildSystemPrompt(PageContext{
		Page:    "backtest",
		Metrics: map[string]interface{}{"sharpe": 1.87, "return": "34.2%"},
	})
	if !strings.Contains(prompt, "backtest") {
		t.Error("expected backtest mention")
	}
	if !strings.Contains(prompt, "sharpe") {
		t.Error("expected metrics in prompt")
	}
}

func TestBuildSystemPrompt_Portfolio(t *testing.T) {
	prompt := BuildSystemPrompt(PageContext{
		Page: "portfolio",
		Positions: []PositionSummary{
			{Symbol: "BTC", PnLPct: 12.5},
			{Symbol: "ETH", PnLPct: -3.2},
		},
	})
	if !strings.Contains(prompt, "2 positions") {
		t.Error("expected position count")
	}
}

func TestBuildSystemPrompt_Default(t *testing.T) {
	prompt := BuildSystemPrompt(PageContext{Page: "unknown"})
	if !strings.Contains(prompt, "general dashboard") {
		t.Error("expected default fallback")
	}
}

func TestBuildSystemPrompt_AlwaysHasSuggestionInstruction(t *testing.T) {
	pages := []string{"chart", "backtest", "portfolio", "monitor", "alerts", "news", "unknown"}
	for _, page := range pages {
		prompt := BuildSystemPrompt(PageContext{Page: page})
		if !strings.Contains(prompt, "suggested follow-up questions") {
			t.Errorf("page %q: expected suggestion instruction in prompt", page)
		}
	}
}
