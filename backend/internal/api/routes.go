package api

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/indicator"
	"github.com/trader-claude/backend/internal/monitor"
	"github.com/trader-claude/backend/internal/portfolio"
	"github.com/trader-claude/backend/internal/price"
	"github.com/trader-claude/backend/internal/replay"
	"github.com/trader-claude/backend/internal/worker"
	"github.com/trader-claude/backend/internal/ws"
)

// RegisterRoutes wires all HTTP and WebSocket routes onto the Fiber app
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager, monMgr *monitor.Manager) {
	// Health
	health := newHealthHandler(db, rdb, version)
	app.Get("/health", health.check)

	// API v1 group
	v1 := app.Group("/api/v1")

	// --- Markets ---
	mh := newMarketsHandler(ds)
	v1.Get("/markets", mh.listAdapters)
	v1.Get("/markets/:adapterID/symbols", mh.listSymbols)

	// --- Candles ---
	v1.Get("/candles/timeframes", mh.listTimeframes)
	v1.Get("/candles", mh.getCandles)

	// --- Strategies ---
	bh := newBacktestHandler(db, rdb, pool, ds)
	v1.Get("/strategies", bh.listStrategies)
	v1.Get("/strategies/:id", bh.getStrategy)

	// --- Backtests ---
	v1.Post("/backtest/run", bh.runBacktest)
	v1.Get("/backtest/runs", bh.listRuns)
	v1.Get("/backtest/runs/:id", bh.getRun)
	v1.Delete("/backtest/runs/:id", bh.deleteRun)

	// --- Replay ---
	rh := newReplayHandler(db, ds, mgr)
	v1.Post("/backtest/runs/:id/replay", rh.createReplay)
	v1.Post("/replay/bookmarks", rh.createBookmark)
	v1.Get("/replay/bookmarks", rh.listBookmarks)
	v1.Get("/replay/bookmarks/:id", rh.getBookmark)
	v1.Delete("/replay/bookmarks/:id", rh.deleteBookmark)

	// --- Indicators ---
	ih := indicator.NewHandler()
	v1.Get("/indicators", ih.ListIndicators)
	v1.Post("/indicators/calculate", ih.Calculate)

	// --- Portfolios ---
	priceSvc := price.NewService(rdb, "", "")
	portfolioSvc := portfolio.NewService(db, priceSvc)
	ph := newPortfolioHandler(portfolioSvc)
	ph.registerRoutes(v1)

	// --- News ---
	nh := newNewsHandler(db)
	v1.Get("/news", nh.listNews)
	v1.Get("/news/symbols/:symbol", nh.newsBySymbol)

	// --- Alerts ---
	ah := newAlertHandler(db, priceSvc)
	v1.Post("/alerts", ah.createAlert)
	v1.Get("/alerts", ah.listAlerts)
	v1.Delete("/alerts/:id", ah.deleteAlert)
	v1.Patch("/alerts/:id/toggle", ah.toggleAlert)

	// --- Notifications ---
	nfh := newNotificationHandler(db, rdb)
	v1.Get("/notifications", nfh.listNotifications)
	v1.Patch("/notifications/:id/read", nfh.markRead)
	v1.Post("/notifications/read-all", nfh.markAllRead)
	v1.Get("/notifications/unread-count", nfh.unreadCount)

	// --- Monitors ---
	mnh := newMonitorHandler(db, monMgr)
	v1.Post("/monitors", mnh.createMonitor)
	v1.Get("/monitors", mnh.listMonitors)
	v1.Get("/monitors/:id", mnh.getMonitor)
	v1.Put("/monitors/:id", mnh.updateMonitor)
	v1.Delete("/monitors/:id", mnh.deleteMonitor)
	v1.Patch("/monitors/:id/toggle", mnh.toggleMonitor)
	v1.Get("/monitors/:id/signals", mnh.listSignals)

	// --- Social Cards ---
	sh := newSocialHandler(db)
	v1.Post("/social/backtest-card/:runId", sh.backtestCard)
	v1.Post("/social/signal-card/:signalId", sh.signalCard)
	v1.Post("/social/send-telegram", sh.sendTelegram)

	// --- Settings ---
	seth := newSettingsHandler(db)
	v1.Get("/settings/notifications", seth.getNotificationSettings)
	v1.Post("/settings/notifications", seth.saveNotificationSettings)
	v1.Post("/settings/notifications/test", seth.testNotificationSettings)

	// --- AI ---
	aih := newAIHandler(db)
	v1.Post("/ai/chat", aih.chat)
	v1.Get("/settings/ai", aih.getAISettings)
	v1.Post("/settings/ai", aih.saveAISettings)
	v1.Post("/settings/ai/test", aih.testAIConnection)

	// --- WebSocket upgrade middleware ---
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Market data WebSocket (hub)
	app.Get("/ws", websocket.New(hub.ServeWS))

	// Backtest progress WebSocket
	app.Get("/ws/backtest/:id/progress", websocket.New(bh.progressWS))

	// Replay WebSocket
	app.Get("/ws/replay/:replay_id", websocket.New(rh.replayWS))

	// Notifications WebSocket
	app.Get("/ws/notifications", websocket.New(nfh.notificationsWS))

	// Monitor signals WebSocket (multiplexed)
	app.Get("/ws/monitors/signals", websocket.New(signalsWS(rdb)))
}
