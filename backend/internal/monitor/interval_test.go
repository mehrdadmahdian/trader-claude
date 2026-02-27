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
	if tfDuration("1h") != time.Hour {
		t.Error("1h should be 1 hour")
	}
	if tfDuration("1d") != 24*time.Hour {
		t.Error("1d should be 24 hours")
	}
	if tfDuration("unknown") != time.Hour {
		t.Error("unknown should default to 1 hour")
	}
}
