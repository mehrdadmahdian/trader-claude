package api

import (
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/indicator"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/monitor"
	"github.com/trader-claude/backend/internal/portfolio"
	"github.com/trader-claude/backend/internal/price"
	"github.com/trader-claude/backend/internal/replay"
	"github.com/trader-claude/backend/internal/worker"
	"github.com/trader-claude/backend/internal/ws"
)

// RegisterRoutes wires all HTTP and WebSocket routes onto the Fiber app
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager, monMgr *monitor.Manager, authSvc *auth.AuthService, corsOrigins string) {
	// Health
	health := newHealthHandler(db, rdb, version)
	app.Get("/health", health.check)

	// API v1 group
	v1 := app.Group("/api/v1")

	// --- Public auth routes ---
	authH := newAuthHandler(authSvc)
	loginLimiter := limiter.New(limiter.Config{
		Max:        5,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many login attempts, try again later"})
		},
	})

	mutationLimiter := limiter.New(limiter.Config{
		Max:        20,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate limit exceeded"})
		},
	})

	v1.Post("/auth/register", authH.register)
	v1.Post("/auth/login", loginLimiter, authH.login)
	v1.Post("/auth/refresh", authH.refresh)

	// --- Protected routes (require valid JWT) ---
	protected := v1.Group("", auth.RequireAuth(authSvc))

	protected.Post("/auth/logout", authH.logout)
	protected.Get("/auth/me", authH.me)
	protected.Put("/auth/me", authH.updateMe)

	// --- Markets ---
	mh := newMarketsHandler(ds)
	protected.Get("/markets", mh.listAdapters)
	protected.Get("/markets/:adapterID/symbols", mh.listSymbols)

	// --- Candles ---
	protected.Get("/candles/timeframes", mh.listTimeframes)
	protected.Get("/candles", mh.getCandles)

	// --- Strategies ---
	bh := newBacktestHandler(db, rdb, pool, ds)
	protected.Get("/strategies", bh.listStrategies)
	protected.Get("/strategies/:id", bh.getStrategy)

	// --- Backtests ---
	protected.Post("/backtest/run", mutationLimiter, bh.runBacktest)
	protected.Get("/backtest/runs", bh.listRuns)
	protected.Get("/backtest/runs/:id", bh.getRun)
	protected.Delete("/backtest/runs/:id", bh.deleteRun)

	// --- Replay ---
	rh := newReplayHandler(db, ds, mgr)
	protected.Post("/backtest/runs/:id/replay", rh.createReplay)
	protected.Post("/replay/bookmarks", rh.createBookmark)
	protected.Get("/replay/bookmarks", rh.listBookmarks)
	protected.Get("/replay/bookmarks/:id", rh.getBookmark)
	protected.Delete("/replay/bookmarks/:id", rh.deleteBookmark)

	// --- Analytics ---
	anah := newAnalyticsHandler(db, pool)
	protected.Get("/backtest/runs/:id/param-heatmap", anah.paramHeatmap)
	protected.Post("/backtest/runs/:id/monte-carlo", anah.monteCarlo)
	protected.Get("/backtest/runs/:id/walk-forward", anah.walkForward)
	protected.Post("/backtest/compare", anah.compareRuns)
	protected.Get("/analytics/jobs/:jobId", anah.getJob)

	// --- Indicators ---
	ih := indicator.NewHandler()
	protected.Get("/indicators", ih.ListIndicators)
	protected.Post("/indicators/calculate", ih.Calculate)

	// --- Portfolios ---
	priceSvc := price.NewService(rdb, "", "")
	portfolioSvc := portfolio.NewService(db, priceSvc)
	ph := newPortfolioHandler(portfolioSvc)
	protected.Post("/portfolios", mutationLimiter, ph.create)
	protected.Get("/portfolios", ph.list)
	protected.Get("/portfolios/:id", ph.get)
	protected.Put("/portfolios/:id", ph.update)
	protected.Delete("/portfolios/:id", ph.delete)
	protected.Get("/portfolios/:id/summary", ph.summary)
	protected.Post("/portfolios/:id/positions", mutationLimiter, ph.addPosition)
	protected.Put("/portfolios/:id/positions/:posId", ph.updatePosition)
	protected.Delete("/portfolios/:id/positions/:posId", ph.deletePosition)
	protected.Post("/portfolios/:id/transactions", mutationLimiter, ph.addTransaction)
	protected.Get("/portfolios/:id/transactions", ph.listTransactions)
	protected.Get("/portfolios/:id/equity-curve", ph.equityCurve)

	// --- News ---
	nh := newNewsHandler(db)
	protected.Get("/news", nh.listNews)
	protected.Get("/news/symbols/:symbol", nh.newsBySymbol)

	// --- Alerts ---
	ah := newAlertHandler(db, priceSvc)
	protected.Post("/alerts", mutationLimiter, ah.createAlert)
	protected.Get("/alerts", ah.listAlerts)
	protected.Delete("/alerts/:id", ah.deleteAlert)
	protected.Patch("/alerts/:id/toggle", ah.toggleAlert)

	// --- Notifications ---
	nfh := newNotificationHandler(db, rdb)
	protected.Get("/notifications", nfh.listNotifications)
	protected.Patch("/notifications/:id/read", nfh.markRead)
	protected.Post("/notifications/read-all", nfh.markAllRead)
	protected.Get("/notifications/unread-count", nfh.unreadCount)

	// --- Monitors ---
	mnh := newMonitorHandler(db, monMgr)
	protected.Post("/monitors", mutationLimiter, mnh.createMonitor)
	protected.Get("/monitors", mnh.listMonitors)
	protected.Get("/monitors/:id", mnh.getMonitor)
	protected.Put("/monitors/:id", mnh.updateMonitor)
	protected.Delete("/monitors/:id", mnh.deleteMonitor)
	protected.Patch("/monitors/:id/toggle", mnh.toggleMonitor)
	protected.Get("/monitors/:id/signals", mnh.listSignals)

	// --- Social Cards ---
	sh := newSocialHandler(db)
	protected.Post("/social/backtest-card/:runId", sh.backtestCard)
	protected.Post("/social/signal-card/:signalId", sh.signalCard)
	protected.Post("/social/send-telegram", sh.sendTelegram)

	// --- Workspaces (Bloomberg terminal) ---
	workspaceH := newWorkspaceHandler(db)
	protected.Get("/workspaces", workspaceH.list)
	protected.Post("/workspaces", workspaceH.create)
	protected.Get("/workspaces/:id", workspaceH.get)
	protected.Put("/workspaces/:id", workspaceH.update)
	protected.Delete("/workspaces/:id", workspaceH.delete)

	// --- Settings ---
	seth := newSettingsHandler(db)
	protected.Get("/settings/notifications", seth.getNotificationSettings)
	protected.Post("/settings/notifications", seth.saveNotificationSettings)
	protected.Post("/settings/notifications/test", seth.testNotificationSettings)

	// --- AI ---
	aih := newAIHandler(db)
	protected.Post("/ai/chat", aih.chat)
	protected.Get("/settings/ai", aih.getAISettings)
	protected.Post("/settings/ai", aih.saveAISettings)
	protected.Post("/settings/ai/test", aih.testAIConnection)

	// --- Admin ---
	adminH := newAdminHandler(db)
	admin := protected.Group("/admin", auth.RequireRole(models.UserRoleAdmin))
	admin.Get("/users", adminH.listUsers)
	admin.Patch("/users/:id/toggle", adminH.toggleUser)
	admin.Patch("/users/:id/role", adminH.changeRole)

	// --- WebSocket upgrade middleware (validates origin and token, sets locals) ---
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			// Origin validation
			origin := c.Get("Origin")
			// Empty Origin is allowed for non-browser clients (tooling, server-to-server).
			// Browser clients always send Origin, so the check below covers them.
			if origin != "" {
				allowedOrigins := strings.Split(corsOrigins, ",")
				allowed := false
				for _, o := range allowedOrigins {
					if strings.TrimSpace(o) == origin {
						allowed = true
						break
					}
				}
				if !allowed {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "origin not allowed"})
				}
			}
			// JWT validation
			token := c.Query("token")
			claims, err := authSvc.ValidateAccessToken(token)
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"type": "error", "message": "unauthorized"})
			}
			c.Locals("user_id", claims.UserID)
			c.Locals("user_email", claims.Email)
			c.Locals("user_role", claims.Role)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Market data WebSocket (hub)
	app.Get("/ws", websocket.New(hub.ServeWS, websocket.Config{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}))

	// Backtest progress WebSocket
	app.Get("/ws/backtest/:id/progress", websocket.New(bh.progressWS, websocket.Config{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}))

	// Replay WebSocket
	app.Get("/ws/replay/:replay_id", websocket.New(rh.replayWS, websocket.Config{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}))

	// Notifications WebSocket
	app.Get("/ws/notifications", websocket.New(nfh.notificationsWS, websocket.Config{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}))

	// Monitor signals WebSocket (multiplexed)
	app.Get("/ws/monitors/signals", websocket.New(signalsWS(rdb), websocket.Config{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}))
}
