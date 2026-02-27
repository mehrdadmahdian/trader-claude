package news

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/trader-claude/backend/internal/models"
)

const fetchInterval = 15 * time.Minute

// Aggregator runs a background goroutine that fetches RSS feeds every 15 minutes,
// de-duplicates by URL via the database UNIQUE index, tags symbols, scores sentiment,
// and persists new articles to the news_items table.
type Aggregator struct {
	db    *gorm.DB
	feeds []Feed
}

// NewAggregator creates an Aggregator. Pass DefaultFeeds for production.
func NewAggregator(db *gorm.DB, feeds []Feed) *Aggregator {
	return &Aggregator{db: db, feeds: feeds}
}

// Start launches the fetch loop in a background goroutine.
// It fetches once immediately, then repeats every 15 minutes.
// Cancel ctx to stop the loop.
func (a *Aggregator) Start(ctx context.Context) {
	go func() {
		a.fetchAndStore(ctx)
		ticker := time.NewTicker(fetchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.fetchAndStore(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// fetchAndStore fetches every feed and upserts new articles.
// Duplicate URLs are silently ignored via INSERT ... ON CONFLICT DO NOTHING.
func (a *Aggregator) fetchAndStore(ctx context.Context) {
	for _, feed := range a.feeds {
		items := fetchFeed(ctx, feed)
		for _, item := range items {
			symbols := ExtractSymbols(item.Title + " " + item.Summary)
			symbolsArr := make(models.JSONArray, len(symbols))
			for i, s := range symbols {
				symbolsArr[i] = s
			}

			record := models.NewsItem{
				URL:         item.URL,
				Title:       item.Title,
				Summary:     item.Summary,
				Source:      item.Source,
				PublishedAt: item.PublishedAt,
				Symbols:     symbolsArr,
				Sentiment:   Score(item.Title + " " + item.Summary),
				FetchedAt:   time.Now(),
			}

			if err := a.db.WithContext(ctx).
				Clauses(clause.OnConflict{DoNothing: true}).
				Create(&record).Error; err != nil {
				log.Printf("news aggregator: failed to save %q: %v", item.URL, err)
			}
		}
	}
}
