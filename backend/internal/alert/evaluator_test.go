package alert

import (
	"testing"

	"github.com/trader-claude/backend/internal/models"
)

func TestCheckCondition_PriceAbove_Triggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceAbove,
		Threshold: 50000.0,
	}
	triggered, msg := checkCondition(a, 51000.0)
	if !triggered {
		t.Error("expected triggered")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestCheckCondition_PriceAbove_NotTriggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceAbove,
		Threshold: 50000.0,
	}
	triggered, _ := checkCondition(a, 49000.0)
	if triggered {
		t.Error("expected not triggered")
	}
}

func TestCheckCondition_PriceBelow_Triggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceBelow,
		Threshold: 50000.0,
	}
	triggered, _ := checkCondition(a, 49000.0)
	if !triggered {
		t.Error("expected triggered")
	}
}

func TestCheckCondition_PriceBelow_NotTriggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceBelow,
		Threshold: 50000.0,
	}
	triggered, _ := checkCondition(a, 51000.0)
	if triggered {
		t.Error("expected not triggered")
	}
}

func TestCheckCondition_PriceChangePct_Up_Triggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceChange,
		Threshold: 5.0,
		BasePrice: 50000.0,
	}
	triggered, msg := checkCondition(a, 55000.0)
	if !triggered {
		t.Error("expected triggered at 10% change")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestCheckCondition_PriceChangePct_NotTriggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceChange,
		Threshold: 5.0,
		BasePrice: 50000.0,
	}
	triggered, _ := checkCondition(a, 51000.0)
	if triggered {
		t.Error("expected not triggered at 2% change")
	}
}

func TestCheckCondition_PriceChangePct_Down_Triggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceChange,
		Threshold: 5.0,
		BasePrice: 50000.0,
	}
	triggered, _ := checkCondition(a, 47000.0)
	if !triggered {
		t.Error("expected triggered at -6% change")
	}
}

func TestCheckCondition_PriceChangePct_ZeroBase(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceChange,
		Threshold: 5.0,
		BasePrice: 0,
	}
	triggered, _ := checkCondition(a, 51000.0)
	if triggered {
		t.Error("expected not triggered when base price is 0")
	}
}
