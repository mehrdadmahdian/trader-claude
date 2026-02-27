package news

import "strings"

var positiveWords = []string{
	"surge", "rally", "soar", "gain", "gains", "rise", "rises", "bull", "bullish",
	"breakout", "adoption", "partnership", "approval", "upgrade", "growth",
	"profit", "high", "record", "milestone", "launch", "support", "recover",
	"recovery", "upside", "outperform", "buy", "accumulate",
}

var negativeWords = []string{
	"crash", "drop", "drops", "fall", "falls", "plunge", "plunges", "bear", "bearish",
	"hack", "ban", "banned", "regulation", "fine", "lawsuit", "sell-off", "selloff",
	"loss", "losses", "low", "decline", "declining", "warning", "risk", "scam",
	"fraud", "bankrupt", "bankruptcy", "collapse", "collapses", "dump", "dumps",
}

// Score returns a sentiment score in the range [-1, 1].
// +1 = fully positive, -1 = fully negative, 0 = neutral.
func Score(text string) float64 {
	lower := strings.ToLower(text)
	pos, neg := 0, 0
	for _, w := range positiveWords {
		if strings.Contains(lower, w) {
			pos++
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(lower, w) {
			neg++
		}
	}
	total := pos + neg
	if total == 0 {
		return 0
	}
	return float64(pos-neg) / float64(total)
}
