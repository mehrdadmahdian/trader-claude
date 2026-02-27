package monitor

import (
	"testing"
	"time"
)

func TestCalcPollInterval(t *testing.T) {
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
		{"unknown", 60 * time.Second},
	}
	for _, tc := range cases {
		got := calcPollInterval(tc.tf)
		if got != tc.want {
			t.Errorf("calcPollInterval(%q) = %v, want %v", tc.tf, got, tc.want)
		}
	}
}

func TestTfDuration(t *testing.T) {
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
