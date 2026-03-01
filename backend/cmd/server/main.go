package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/trader-claude/backend/internal/adapter"
	alertpkg "github.com/trader-claude/backend/internal/alert"
	"github.com/trader-claude/backend/internal/api"
	"github.com/trader-claude/backend/internal/config"
	"github.com/trader-claude/backend/internal/models"
	monpkg "github.com/trader-claude/backend/internal/monitor"
	"github.com/trader-claude/backend/internal/news"
	"github.com/trader-claude/backend/internal/price"
	"github.com/trader-claude/backend/internal/registry"
	"github.com/trader-claude/backend/internal/replay"
	"github.com/trader-claude/backend/internal/strategy"
	"github.com/trader-claude/backend/internal/worker"
	"github.com/trader-claude/backend/internal/ws"
)

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("starting trader-claude %s (env=%s)", cfg.App.Version, cfg.App.Env)

	// 2. Connect MySQL via GORM
	gormCfg := &gorm.Config{}
	if cfg.App.Env == "production" {
		gormCfg.Logger = gormlogger.Default.LogMode(gormlogger.Warn)
	} else {
		gormCfg.Logger = gormlogger.Default.LogMode(gormlogger.Info)
	}

	db, err := connectDB(cfg.DB.DSN, gormCfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("database connected")

	// Run GORM auto-migrate
	if err := autoMigrate(db); err != nil {
		log.Fatalf("auto-migrate failed: %v", err)
	}
	log.Println("database migrated")

	// 3. Connect Redis
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	log.Println("redis connected")

	// 4. Register market adapters
	registry.Adapters().Register(adapter.NewBinanceAdapter(""))
	registry.Adapters().Register(adapter.NewYahooAdapter())
	log.Printf("registered adapters: %v", registry.Adapters().Names())

	// 4b. Register strategies
	registry.Strategies().Register("ema_crossover", func() registry.Strategy { return &strategy.EMACrossover{} })
	registry.Strategies().Register("rsi", func() registry.Strategy { return &strategy.RSIStrategy{} })
	registry.Strategies().Register("macd", func() registry.Strategy { return &strategy.MACDSignal{} })
	log.Printf("registered strategies: %v", registry.Strategies().Names())

	// Start data sync worker (tracks recently accessed symbols)
	ds := adapter.NewDataService(db, rdb)
	ds.StartSyncWorker(context.Background(), func(name string) (registry.MarketAdapter, bool) {
		a, err := registry.Adapters().Get(name)
		return a, err == nil
	})

	// 5. Initialize WebSocket hub
	hub := ws.NewHub()
	go hub.Run()

	// 5. Initialize worker pool
	pool := worker.NewPool(cfg.Worker.PoolSize)
	pool.Start()

	// Initialize replay manager
	replayMgr := replay.NewManager()

	// Start news aggregator (fetches RSS every 15 min)
	newsAgg := news.NewAggregator(db, news.DefaultFeeds)
	newsAgg.Start(context.Background())

	// Start alert evaluator (checks active alerts every 60 s)
	priceSvcMain := price.NewService(rdb, "", "")
	alertEval := alertpkg.NewEvaluator(db, priceSvcMain, rdb)
	alertEval.Start(context.Background())

	// Start monitor manager
	monitorMgr := monpkg.NewManager(db, rdb, ds, pool)
	monitorMgr.Start(context.Background())

	// 6. Setup Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "trader-claude " + cfg.App.Version,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.Origins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	// 7. Register routes
	api.RegisterRoutes(app, db, rdb, hub, cfg.App.Version, pool, ds, replayMgr, monitorMgr)

	// 8. Start server
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	go func() {
		log.Printf("listening on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Printf("server error: %v", err)
		}
	}()

	// 9. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	monitorMgr.Stop()
	pool.Stop()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	if err := app.ShutdownWithContext(shutCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	_ = rdb.Close()
	log.Println("bye")
}

func connectDB(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	// Retry logic — MySQL container may not be ready immediately
	for i := 0; i < 10; i++ {
		db, err = gorm.Open(mysql.Open(dsn), cfg)
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				sqlDB.SetMaxIdleConns(10)
				sqlDB.SetMaxOpenConns(100)
				sqlDB.SetConnMaxLifetime(time.Hour)
				return db, nil
			}
		}
		log.Printf("waiting for database (attempt %d/10)...", i+1)
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to database after 10 attempts: %w", err)
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Symbol{},
		&models.Candle{},
		&models.StrategyDef{},
		&models.Backtest{},
		&models.Trade{},
		&models.Portfolio{},
		&models.Position{},
		&models.Transaction{},
		&models.Alert{},
		&models.Notification{},
		&models.NewsItem{},
		&models.WatchList{},
		&models.ReplayBookmark{},
		&models.Monitor{},
		&models.MonitorSignal{},
		&models.Setting{},
		&models.AnalyticsResult{},
	)
}
