package validation

import (
	"testing"
)

type testStruct struct {
	Name   string `validate:"required,safe_string"`
	Symbol string `validate:"required,symbol"`
	TF     string `validate:"required,timeframe"`
	Market string `validate:"required,market"`
}

func TestValidate_ValidInput(t *testing.T) {
	s := testStruct{Name: "My Backtest", Symbol: "BTC/USDT", TF: "1h", Market: "crypto"}
	if err := Validate(s); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidate_ScriptInjection(t *testing.T) {
	s := testStruct{Name: "<script>alert('xss')</script>", Symbol: "BTC/USDT", TF: "1h", Market: "crypto"}
	if err := Validate(s); err == nil {
		t.Error("expected error for script injection")
	}
}

func TestValidate_SQLInjection(t *testing.T) {
	s := testStruct{Name: "'; DROP TABLE users; --", Symbol: "BTC/USDT", TF: "1h", Market: "crypto"}
	if err := Validate(s); err == nil {
		t.Error("expected error for SQL injection")
	}
}

func TestValidate_InvalidSymbol(t *testing.T) {
	s := testStruct{Name: "Test", Symbol: "invalid symbol!", TF: "1h", Market: "crypto"}
	if err := Validate(s); err == nil {
		t.Error("expected error for invalid symbol")
	}
}

func TestValidate_ValidSymbolNoSlash(t *testing.T) {
	s := testStruct{Name: "Test", Symbol: "AAPL", TF: "1d", Market: "stock"}
	if err := Validate(s); err != nil {
		t.Errorf("expected valid for AAPL, got: %v", err)
	}
}

func TestValidate_InvalidTimeframe(t *testing.T) {
	s := testStruct{Name: "Test", Symbol: "BTC/USDT", TF: "2h", Market: "crypto"}
	if err := Validate(s); err == nil {
		t.Error("expected error for invalid timeframe")
	}
}

func TestValidate_InvalidMarket(t *testing.T) {
	s := testStruct{Name: "Test", Symbol: "BTC/USDT", TF: "1h", Market: "bonds"}
	if err := Validate(s); err == nil {
		t.Error("expected error for invalid market")
	}
}
