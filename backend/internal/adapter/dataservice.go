package adapter

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
)

const (
	recentSymbolsKey = "market:recent_symbols"
	recentWindowHours = 24
	syncIntervalMin   = 5
	syncRecentCount   = 500
)

// DataService provides smart candle retrieval with gap-filling.
// It queries the DB first, fetches missing ranges from the adapter,
// upserts, and returns merged data.
type DataService struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewDataService creates a DataService.
func NewDataService(db *gorm.DB, rdb *redis.Client) *DataService {
	return &DataService{db: db, rdb: rdb}
}

// GetCandles returns OHLCV candles for the given parameters.
// It queries the DB, fills gaps by fetching from the adapter, upserts, and
// returns all candles in [from, to] sorted ascending.
func (s *DataService) GetCandles(
	ctx context.Context,
	adapter registry.MarketAdapter,
	symbol, market, timeframe string,
	from, to time.Time,
) ([]models.Candle, error) {
	// Track this symbol for background sync
	s.trackAccess(ctx, adapter.Name(), symbol, market, timeframe)

	// 1. Load existing candles from DB
	existing, err := s.queryDB(ctx, symbol, market, timeframe, from, to)
	if err != nil {
		return nil, fmt.Errorf("dataservice GetCandles: query db: %w", err)
	}

	// 2. Find time gaps
	tfDur := timeframeDuration(timeframe)
	gaps := findGaps(existing, from, to, tfDur)

	// 3. Fetch each gap from the adapter and upsert
	for _, gap := range gaps {
		fetched, err := adapter.FetchCandles(ctx, symbol, market, timeframe, gap.from, gap.to)
		if err != nil {
			log.Printf("dataservice: fetch gap [%v, %v]: %v", gap.from, gap.to, err)
			continue // best-effort: don't fail the whole request
		}
		if len(fetched) == 0 {
			continue
		}
		if err := s.upsert(ctx, fetched); err != nil {
			log.Printf("dataservice: upsert: %v", err)
		}
	}

	// 4. Re-query to get the final merged result
	if len(gaps) > 0 {
		existing, err = s.queryDB(ctx, symbol, market, timeframe, from, to)
		if err != nil {
			return nil, fmt.Errorf("dataservice GetCandles: re-query db: %w", err)
		}
	}

	return existing, nil
}

// SyncRecent fetches the most recent `syncRecentCount` candles and upserts them.
// Called by the background sync worker.
func (s *DataService) SyncRecent(
	ctx context.Context,
	adapter registry.MarketAdapter,
	symbol, market, timeframe string,
) error {
	tfDur := timeframeDuration(timeframe)
	to := time.Now().UTC()
	from := to.Add(-time.Duration(syncRecentCount) * tfDur)

	candles, err := adapter.FetchCandles(ctx, symbol, market, timeframe, from, to)
	if err != nil {
		return fmt.Errorf("dataservice SyncRecent: fetch: %w", err)
	}
	if len(candles) == 0 {
		return nil
	}
	return s.upsert(ctx, candles)
}

// StartSyncWorker starts a background goroutine that syncs recently accessed
// symbols every syncIntervalMin minutes. Call this after initialising the service.
// The goroutine exits when ctx is cancelled.
func (s *DataService) StartSyncWorker(ctx context.Context, adapters func(name string) (registry.MarketAdapter, bool)) {
	go func() {
		ticker := time.NewTicker(time.Duration(syncIntervalMin) * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.syncRecentlyAccessed(ctx, adapters)
			}
		}
	}()
}

// --- internal ---

type timeRange struct {
	from, to time.Time
}

// findGaps returns the time ranges within [from, to] that are not covered by
// the existing candles. A gap is detected when two consecutive candles are
// more than 1.5× the timeframe apart.
func findGaps(candles []models.Candle, from, to time.Time, tfDur time.Duration) []timeRange {
	if tfDur == 0 {
		return nil
	}
	threshold := time.Duration(float64(tfDur) * 1.5)

	var gaps []timeRange

	if len(candles) == 0 {
		return []timeRange{{from: from, to: to}}
	}

	// Gap before the first candle
	if candles[0].Timestamp.Sub(from) > threshold {
		gaps = append(gaps, timeRange{
			from: from,
			to:   candles[0].Timestamp.Add(-tfDur),
		})
	}

	// Gaps between consecutive candles
	for i := 1; i < len(candles); i++ {
		diff := candles[i].Timestamp.Sub(candles[i-1].Timestamp)
		if diff > threshold {
			gaps = append(gaps, timeRange{
				from: candles[i-1].Timestamp.Add(tfDur),
				to:   candles[i].Timestamp.Add(-tfDur),
			})
		}
	}

	// Gap after the last candle
	last := candles[len(candles)-1].Timestamp
	if to.Sub(last) > threshold {
		gaps = append(gaps, timeRange{
			from: last.Add(tfDur),
			to:   to,
		})
	}

	return gaps
}

// timeframeDuration returns the wall-clock duration of a timeframe string.
func timeframeDuration(tf string) time.Duration {
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
		return 0
	}
}

func (s *DataService) queryDB(
	ctx context.Context,
	symbol, market, timeframe string,
	from, to time.Time,
) ([]models.Candle, error) {
	var candles []models.Candle
	err := s.db.WithContext(ctx).
		Where("symbol = ? AND market = ? AND timeframe = ? AND timestamp >= ? AND timestamp <= ?",
			symbol, market, timeframe, from, to).
		Order("timestamp ASC").
		Find(&candles).Error
	return candles, err
}

// upsert inserts or updates candles in the DB using INSERT ... ON DUPLICATE KEY UPDATE.
func (s *DataService) upsert(ctx context.Context, candles []registry.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	rows := make([]models.Candle, 0, len(candles))
	for _, c := range candles {
		rows = append(rows, models.Candle{
			Symbol:    c.Symbol,
			Market:    c.Market,
			Timeframe: c.Timeframe,
			Timestamp: c.Timestamp,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		})
	}

	// Upsert in batches of 500 to avoid giant queries
	const batchSize = 500
	for i := 0; i < len(rows); i += batchSize {
		end := i + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[i:end]
		err := s.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "symbol"}, {Name: "market"}, {Name: "timeframe"}, {Name: "timestamp"}},
				DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume"}),
			}).
			Create(&batch).Error
		if err != nil {
			return fmt.Errorf("upsert batch: %w", err)
		}
	}
	return nil
}

// trackAccess records that the symbol+timeframe was accessed, for sync scheduling.
func (s *DataService) trackAccess(ctx context.Context, adapterName, symbol, market, timeframe string) {
	key := fmt.Sprintf("%s:%s:%s:%s", adapterName, symbol, market, timeframe)
	_ = s.rdb.ZAdd(ctx, recentSymbolsKey, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: key,
	}).Err()
}

// syncRecentlyAccessed fetches all symbols accessed in the last 24h from Redis
// and calls SyncRecent for each.
func (s *DataService) syncRecentlyAccessed(ctx context.Context, getAdapter func(name string) (registry.MarketAdapter, bool)) {
	cutoff := float64(time.Now().Add(-recentWindowHours * time.Hour).Unix())

	members, err := s.rdb.ZRangeByScore(ctx, recentSymbolsKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", cutoff),
		Max: "+inf",
	}).Result()
	if err != nil {
		log.Printf("dataservice sync: read recent symbols: %v", err)
		return
	}

	for _, member := range members {
		// Parse "adapterName:symbol:market:timeframe"
		parts := splitKey(member, 4)
		if parts == nil {
			continue
		}
		adapterName, symbol, market, timeframe := parts[0], parts[1], parts[2], parts[3]

		adapter, ok := getAdapter(adapterName)
		if !ok {
			continue
		}

		if err := s.SyncRecent(ctx, adapter, symbol, market, timeframe); err != nil {
			log.Printf("dataservice sync: %s %s/%s: %v", adapterName, symbol, timeframe, err)
		}
	}
}

// splitKey splits "a:b:c:d" into ["a","b","c","d"], returns nil if wrong count.
func splitKey(s string, n int) []string {
	parts := make([]string, 0, n)
	start := 0
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			count++
			if count == n-1 {
				parts = append(parts, s[start:i])
				parts = append(parts, s[i+1:])
				return parts
			}
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if len(parts) != n {
		return nil
	}
	return parts
}

// mergeAndSort merges two candle slices and returns them sorted by timestamp.
// Used internally when we need to combine DB results with freshly fetched data.
func mergeAndSort(a, b []models.Candle) []models.Candle {
	merged := make([]models.Candle, 0, len(a)+len(b))
	merged = append(merged, a...)
	merged = append(merged, b...)
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp.Before(merged[j].Timestamp)
	})
	// Deduplicate by timestamp
	if len(merged) == 0 {
		return merged
	}
	deduped := merged[:1]
	for _, c := range merged[1:] {
		if !c.Timestamp.Equal(deduped[len(deduped)-1].Timestamp) {
			deduped = append(deduped, c)
		}
	}
	return deduped
}
