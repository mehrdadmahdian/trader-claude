package replay

import (
	"testing"
)

func TestManager_StoreAndRetrieve(t *testing.T) {
	m := NewManager()
	candles := makeCandles(5)
	s := NewSession("abc", 1, candles, makeEquity(candles), nil)

	m.Store(s)
	got, ok := m.Get("abc")
	if !ok {
		t.Fatal("expected to find session abc")
	}
	if got.ID != "abc" {
		t.Errorf("expected ID=abc, got %s", got.ID)
	}
}

func TestManager_Delete(t *testing.T) {
	m := NewManager()
	candles := makeCandles(3)
	s := NewSession("xyz", 2, candles, makeEquity(candles), nil)
	m.Store(s)

	m.Delete("xyz")
	_, ok := m.Get("xyz")
	if ok {
		t.Error("expected session xyz to be deleted")
	}
}

func TestManager_GetMissing_ReturnsFalse(t *testing.T) {
	m := NewManager()
	_, ok := m.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent session")
	}
}
