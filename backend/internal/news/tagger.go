package news

import "strings"

type symbolEntry struct {
	symbol  string
	aliases []string
}

var knownSymbols = []symbolEntry{
	{symbol: "BTCUSDT", aliases: []string{"bitcoin", "btc"}},
	{symbol: "ETHUSDT", aliases: []string{"ethereum", "eth"}},
	{symbol: "SOLUSDT", aliases: []string{"solana", "sol"}},
	{symbol: "BNBUSDT", aliases: []string{"binance coin", "bnb"}},
	{symbol: "XRPUSDT", aliases: []string{"ripple", "xrp"}},
	{symbol: "ADAUSDT", aliases: []string{"cardano", "ada"}},
	{symbol: "DOGEUSDT", aliases: []string{"dogecoin", "doge"}},
	{symbol: "AVAXUSDT", aliases: []string{"avalanche", "avax"}},
	{symbol: "DOTUSDT", aliases: []string{"polkadot", "dot"}},
	{symbol: "MATICUSDT", aliases: []string{"polygon", "matic"}},
	{symbol: "AAPL", aliases: []string{"apple", "apple inc", "aapl"}},
	{symbol: "MSFT", aliases: []string{"microsoft", "msft"}},
	{symbol: "SPY", aliases: []string{"s&p 500", "s&p500", "spy etf"}},
	{symbol: "TSLA", aliases: []string{"tesla", "tsla"}},
	{symbol: "NVDA", aliases: []string{"nvidia", "nvda"}},
	{symbol: "GOOGL", aliases: []string{"google", "alphabet", "googl"}},
	{symbol: "AMZN", aliases: []string{"amazon", "amzn"}},
	{symbol: "META", aliases: []string{"meta platforms", "facebook"}},
}

// ExtractSymbols scans text for known tickers and aliases and returns
// a de-duplicated slice of canonical symbol strings (e.g. "BTCUSDT").
// Matching is case-insensitive. Returns nil if no symbols are found.
func ExtractSymbols(text string) []string {
	lower := strings.ToLower(text)
	seen := make(map[string]bool)
	var result []string

	for _, entry := range knownSymbols {
		if seen[entry.symbol] {
			continue
		}
		for _, alias := range entry.aliases {
			if strings.Contains(lower, alias) {
				seen[entry.symbol] = true
				result = append(result, entry.symbol)
				break
			}
		}
	}
	return result
}
