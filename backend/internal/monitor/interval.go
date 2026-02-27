package monitor

import "time"

// calcPollInterval returns how often a monitor should poll for new candles.
// Rule: timeframe / 10, minimum 30s.
func calcPollInterval(timeframe string) time.Duration {
	switch timeframe {
	case "1m":
		return 30 * time.Second
	case "5m":
		return 30 * time.Second
	case "15m":
		return 90 * time.Second
	case "1h":
		return 6 * time.Minute
	case "4h":
		return 24 * time.Minute
	case "1d":
		return 1 * time.Hour
	default:
		return 60 * time.Second
	}
}

// tfDuration returns the wall-clock duration of a timeframe string.
// Used to compute the warm-start candle window.
func tfDuration(tf string) time.Duration {
	switch tf {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	default:
		return time.Hour
	}
}
