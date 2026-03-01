package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/trader-claude/backend/internal/config"
	"github.com/trader-claude/backend/internal/models"
)

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("seed: connecting to database (env=%s)", cfg.App.Env)

	// 2. Connect to MySQL via GORM
	gormCfg := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	}

	db, err := connectDB(cfg.DB.DSN, gormCfg)
	if err != nil {
		log.Fatalf("seed: failed to connect to database: %v", err)
	}
	log.Println("seed: database connected")

	// 3. Run AutoMigrate for required models
	if err := autoMigrate(db); err != nil {
		log.Fatalf("seed: auto-migrate failed: %v", err)
	}
	log.Println("seed: auto-migrate complete")

	// 4. Seed data in order
	portfolioID := seedPortfolio(db)
	seedPositions(db, portfolioID)
	seedMonitor(db)
	seedAlerts(db)
	seedNews(db)

	log.Println("seed: done")
}

// connectDB opens a GORM connection with retry logic.
func connectDB(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	for i := 0; i < 10; i++ {
		db, err = gorm.Open(mysql.Open(dsn), cfg)
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				sqlDB.SetMaxIdleConns(5)
				sqlDB.SetMaxOpenConns(20)
				sqlDB.SetConnMaxLifetime(time.Hour)
				return db, nil
			}
		}
		log.Printf("seed: waiting for database (attempt %d/10)...", i+1)
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to database after 10 attempts: %w", err)
}

// autoMigrate ensures the required tables exist.
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Portfolio{},
		&models.Position{},
		&models.Monitor{},
		&models.Alert{},
		&models.NewsItem{},
	)
}

// seedPortfolio inserts one demo portfolio and returns its ID.
// It skips insertion if a portfolio named "Demo Portfolio" already exists.
func seedPortfolio(db *gorm.DB) int64 {
	var count int64
	if err := db.Model(&models.Portfolio{}).Where("name = ?", "Demo Portfolio").Count(&count).Error; err != nil {
		log.Printf("seed: error checking portfolio count: %v", err)
		return 0
	}
	if count > 0 {
		log.Println("seed: portfolio already exists, skipping")
		var existing models.Portfolio
		if err := db.Where("name = ?", "Demo Portfolio").First(&existing).Error; err != nil {
			log.Printf("seed: error fetching existing portfolio: %v", err)
			return 0
		}
		return existing.ID
	}

	portfolio := models.Portfolio{
		Name:        "Demo Portfolio",
		Description: "A demo portfolio with sample positions for development",
		Type:        models.PortfolioTypeManual,
		Currency:    "USD",
		IsActive:    true,
		InitialCash: 50000.0,
		CurrentCash: 42000.0,
	}

	if err := db.Create(&portfolio).Error; err != nil {
		log.Printf("seed: error creating portfolio: %v", err)
		return 0
	}
	log.Printf("seed: created portfolio id=%d name=%q", portfolio.ID, portfolio.Name)
	return portfolio.ID
}

// seedPositions inserts 3 positions into the given portfolio.
// It skips if positions already exist for that portfolio.
func seedPositions(db *gorm.DB, portfolioID int64) {
	if portfolioID == 0 {
		log.Println("seed: skipping positions — no valid portfolio ID")
		return
	}

	var count int64
	if err := db.Model(&models.Position{}).Where("portfolio_id = ?", portfolioID).Count(&count).Error; err != nil {
		log.Printf("seed: error checking positions count: %v", err)
		return
	}
	if count > 0 {
		log.Printf("seed: positions already exist for portfolio %d, skipping", portfolioID)
		return
	}

	now := time.Now().UTC()

	positions := []models.Position{
		{
			PortfolioID:  portfolioID,
			AdapterID:    "binance",
			Symbol:       "BTCUSDT",
			Market:       "crypto",
			Quantity:     0.5,
			AvgCost:      40000.0,
			CurrentPrice: 40000.0,
			CurrentValue: 20000.0,
			OpenedAt:     now,
		},
		{
			PortfolioID:  portfolioID,
			AdapterID:    "binance",
			Symbol:       "ETHUSDT",
			Market:       "crypto",
			Quantity:     5.0,
			AvgCost:      2200.0,
			CurrentPrice: 2200.0,
			CurrentValue: 11000.0,
			OpenedAt:     now,
		},
		{
			PortfolioID:  portfolioID,
			AdapterID:    "yahoo",
			Symbol:       "AAPL",
			Market:       "stock",
			Quantity:     10.0,
			AvgCost:      175.0,
			CurrentPrice: 175.0,
			CurrentValue: 1750.0,
			OpenedAt:     now,
		},
	}

	for i := range positions {
		if err := db.Create(&positions[i]).Error; err != nil {
			log.Printf("seed: error creating position %s: %v", positions[i].Symbol, err)
			continue
		}
		log.Printf("seed: created position id=%d symbol=%s market=%s", positions[i].ID, positions[i].Symbol, positions[i].Market)
	}
}

// seedMonitor inserts one active monitor configuration.
// It skips if a monitor named "BTC EMA Watch" already exists.
func seedMonitor(db *gorm.DB) {
	var count int64
	if err := db.Model(&models.Monitor{}).Where("name = ?", "BTC EMA Watch").Count(&count).Error; err != nil {
		log.Printf("seed: error checking monitor count: %v", err)
		return
	}
	if count > 0 {
		log.Println("seed: monitor already exists, skipping")
		return
	}

	monitor := models.Monitor{
		Name:         "BTC EMA Watch",
		AdapterID:    "binance",
		Symbol:       "BTCUSDT",
		Market:       "crypto",
		Timeframe:    "1h",
		StrategyName: "ema_crossover",
		Status:       models.MonitorStatusActive,
		Mode:         models.MonitorModeLive,
		NotifyInApp:  true,
	}

	if err := db.Create(&monitor).Error; err != nil {
		log.Printf("seed: error creating monitor: %v", err)
		return
	}
	log.Printf("seed: created monitor id=%d name=%q", monitor.ID, monitor.Name)
}

// seedAlerts inserts 2 price alerts.
// It skips if alerts already exist for BTCUSDT and ETHUSDT with the same conditions.
func seedAlerts(db *gorm.DB) {
	alertDefs := []struct {
		name      string
		symbol    string
		market    string
		condition models.AlertCondition
		threshold float64
	}{
		{
			name:      "BTC Above 100k",
			symbol:    "BTCUSDT",
			market:    "crypto",
			condition: models.AlertConditionPriceAbove,
			threshold: 100000.0,
		},
		{
			name:      "ETH Below 2000",
			symbol:    "ETHUSDT",
			market:    "crypto",
			condition: models.AlertConditionPriceBelow,
			threshold: 2000.0,
		},
	}

	for _, def := range alertDefs {
		var count int64
		if err := db.Model(&models.Alert{}).
			Where("name = ? AND symbol = ?", def.name, def.symbol).
			Count(&count).Error; err != nil {
			log.Printf("seed: error checking alert count for %s: %v", def.symbol, err)
			continue
		}
		if count > 0 {
			log.Printf("seed: alert %q already exists, skipping", def.name)
			continue
		}

		alert := models.Alert{
			Name:            def.name,
			AdapterID:       "binance",
			Symbol:          def.symbol,
			Market:          def.market,
			Condition:       def.condition,
			Threshold:       def.threshold,
			Status:          models.AlertStatusActive,
			RecurringEnabled: true,
			CooldownMinutes: 60,
		}

		if err := db.Create(&alert).Error; err != nil {
			log.Printf("seed: error creating alert for %s: %v", def.symbol, err)
			continue
		}
		log.Printf("seed: created alert id=%d symbol=%s condition=%s threshold=%.2f",
			alert.ID, alert.Symbol, alert.Condition, alert.Threshold)
	}
}

// seedNews inserts 5 realistic news items spread over the last 7 days.
func seedNews(db *gorm.DB) {

	now := time.Now().UTC()

	newsItems := []models.NewsItem{
		{
			URL:         "https://example.com/news/btc-hits-new-high-2026",
			Title:       "Bitcoin Surges Past $95,000 as Institutional Demand Rises",
			Summary:     "Bitcoin reached a new multi-month high as large institutional buyers continued accumulating, with on-chain data showing record inflows.",
			Source:      "CryptoNews",
			PublishedAt: now.Add(-1 * 24 * time.Hour),
			Symbols:     models.JSONArray{"BTCUSDT"},
			Sentiment:   0.82,
			FetchedAt:   now,
		},
		{
			URL:         "https://example.com/news/eth-upgrade-2026",
			Title:       "Ethereum's Latest Network Upgrade Reduces Gas Fees by 40%",
			Summary:     "The Ethereum Foundation announced a successful network upgrade that significantly reduces transaction costs, making DeFi more accessible.",
			Source:      "BlockchainInsider",
			PublishedAt: now.Add(-2 * 24 * time.Hour),
			Symbols:     models.JSONArray{"ETHUSDT"},
			Sentiment:   0.75,
			FetchedAt:   now,
		},
		{
			URL:         "https://example.com/news/aapl-earnings-beat-2026",
			Title:       "Apple Reports Record Q1 Earnings, Services Revenue Up 18%",
			Summary:     "Apple Inc. surpassed analyst expectations with record quarterly revenue driven by strong iPhone sales and continued growth in its services segment.",
			Source:      "MarketWatch",
			PublishedAt: now.Add(-3 * 24 * time.Hour),
			Symbols:     models.JSONArray{"AAPL"},
			Sentiment:   0.70,
			FetchedAt:   now,
		},
		{
			URL:         "https://example.com/news/fed-rate-hold-2026",
			Title:       "Fed Holds Rates Steady, Markets Rally on Dovish Outlook",
			Summary:     "The Federal Reserve kept interest rates unchanged at its latest meeting, citing stable inflation data. Equity and crypto markets rallied on expectations of future rate cuts.",
			Source:      "Reuters",
			PublishedAt: now.Add(-5 * 24 * time.Hour),
			Symbols:     models.JSONArray{"BTCUSDT", "ETHUSDT", "AAPL"},
			Sentiment:   0.60,
			FetchedAt:   now,
		},
		{
			URL:         "https://example.com/news/crypto-regulation-2026",
			Title:       "EU Finalises Crypto Regulatory Framework, Industry Welcomes Clarity",
			Summary:     "The European Union published its finalised MiCA implementation guidelines, providing a clear legal path for crypto exchanges and asset issuers operating in the region.",
			Source:      "CoinDesk",
			PublishedAt: now.Add(-7 * 24 * time.Hour),
			Symbols:     models.JSONArray{"BTCUSDT", "ETHUSDT"},
			Sentiment:   0.55,
			FetchedAt:   now,
		},
	}

	for i := range newsItems {
		// Use FirstOrCreate to avoid unique constraint violations on URL
		result := db.Where("url = ?", newsItems[i].URL).FirstOrCreate(&newsItems[i])
		if result.Error != nil {
			log.Printf("seed: error creating news item %q: %v", newsItems[i].Title, result.Error)
			continue
		}
		if result.RowsAffected == 0 {
			log.Printf("seed: news item already exists, skipping: %q", newsItems[i].Title)
			continue
		}
		log.Printf("seed: created news item id=%d source=%s title=%q", newsItems[i].ID, newsItems[i].Source, newsItems[i].Title)
	}
}
