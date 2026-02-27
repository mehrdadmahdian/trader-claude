package monitor

import (
	"context"
	"testing"
	"time"
)

func TestManagerCalcPollIntervalBuckets(t *testing.T) {
	cases := []struct {
		tf   string
		want time.Duration
	}{
		{"1m", 30 * time.Second},
		{"5m", 30 * time.Second},
		{"15m", 90 * time.Second},
		{"1h", 6 * time.Minute},
		{"4h", 24 * time.Minute},
		{"1d", 1 * time.Hour},
		{"bogus", 60 * time.Second},
	}
	for _, tc := range cases {
		got := calcPollInterval(tc.tf)
		if got != tc.want {
			t.Errorf("calcPollInterval(%q) = %v, want %v", tc.tf, got, tc.want)
		}
	}
}

func TestManagerTfDurationBuckets(t *testing.T) {
	cases := []struct {
		tf   string
		want time.Duration
	}{
		{"1m", time.Minute},
		{"5m", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
		{"4h", 4 * time.Hour},
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"unknown", time.Hour},
	}
	for _, tc := range cases {
		got := tfDuration(tc.tf)
		if got != tc.want {
			t.Errorf("tfDuration(%q) = %v, want %v", tc.tf, got, tc.want)
		}
	}
}

func TestManagerCancelTimerRemovesEntry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		ctx:    ctx,
		cancel: cancel,
		timers: make(map[int64]*time.Timer),
	}

	mgr.mu.Lock()
	mgr.timers[42] = time.AfterFunc(10*time.Minute, func() {})
	mgr.mu.Unlock()

	mgr.Pause(42)

	mgr.mu.Lock()
	_, exists := mgr.timers[42]
	mgr.mu.Unlock()

	if exists {
		t.Error("expected timer 42 to be removed after Pause")
	}
}

func TestManagerRemoveNonExistentIDDoesNotPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		ctx:    ctx,
		cancel: cancel,
		timers: make(map[int64]*time.Timer),
	}
	// Should not panic
	mgr.Remove(999)
}
