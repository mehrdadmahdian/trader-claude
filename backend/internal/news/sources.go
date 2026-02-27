package news

import (
	"context"
	"time"

	"github.com/mmcdole/gofeed"
)

// Feed defines a single RSS news source.
type Feed struct {
	Name string
	URL  string
}

// DefaultFeeds is the list of RSS feeds the aggregator polls.
var DefaultFeeds = []Feed{
	{Name: "CoinDesk", URL: "https://www.coindesk.com/arc/outboundfeeds/rss/"},
	{Name: "CoinTelegraph", URL: "https://cointelegraph.com/rss"},
	{Name: "Reuters Business", URL: "https://feeds.reuters.com/reuters/businessNews"},
}

// FeedItem is a normalized article from any RSS feed.
type FeedItem struct {
	URL         string
	Title       string
	Summary     string
	Source      string
	PublishedAt time.Time
}

// fetchFeed downloads and parses a single RSS feed.
// Returns an empty slice on any error so the caller can continue
// processing other feeds without interruption.
func fetchFeed(ctx context.Context, feed Feed) []FeedItem {
	fp := gofeed.NewParser()
	parsed, err := fp.ParseURLWithContext(feed.URL, ctx)
	if err != nil {
		return nil
	}

	items := make([]FeedItem, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		if item.Link == "" || item.Title == "" {
			continue
		}
		pub := time.Now()
		if item.PublishedParsed != nil {
			pub = *item.PublishedParsed
		}
		summary := item.Description
		if summary == "" {
			summary = item.Content
		}
		items = append(items, FeedItem{
			URL:         item.Link,
			Title:       item.Title,
			Summary:     summary,
			Source:      feed.Name,
			PublishedAt: pub,
		})
	}
	return items
}
