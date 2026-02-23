# Architecture

## Overview

trader-claude is a monorepo with four Docker-managed services that communicate over a shared bridge network (`trader_net`).

```
┌─────────────────────────────────────────────────────────┐
│                     Docker Network: trader_net           │
│                                                         │
│  ┌──────────────┐    ┌──────────────┐                   │
│  │  trader_mysql │    │ trader_redis │                   │
│  │  MySQL 8.0   │    │  Redis 7     │                   │
│  │  :3306       │    │  :6379       │                   │
│  └──────┬───────┘    └──────┬───────┘                   │
│         │                   │                           │
│  ┌──────▼───────────────────▼───────┐                   │
│  │       trader_backend              │                   │
│  │       Go 1.24 + Fiber v2          │                   │
│  │       :8080                       │                   │
│  └──────────────┬────────────────────┘                   │
│                 │                                        │
│  ┌──────────────▼────────────────────┐                   │
│  │       trader_frontend             │                   │
│  │       React 18 + Vite             │                   │
│  │       :5173                       │                   │
│  └───────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────┘
```

## Backend Internal Architecture

```
main.go
  │
  ├── config.Load()           env → Config struct
  ├── db.Connect()            GORM MySQL with retry
  ├── db.AutoMigrate()        create/alter tables from models
  ├── redis.Connect()         go-redis client
  ├── ws.GetHub()             singleton WebSocket hub
  ├── worker.NewPool()        goroutine pool (size from config)
  ├── fiber.New()             HTTP server + middleware
  └── api.RegisterRoutes()    mount all handlers
        │
        ├── GET  /health
        ├── GET  /ws           → ws.Hub
        └── /api/v1/
              ├── /symbols
              ├── /candles
              ├── /backtests
              ├── /strategies
              ├── /portfolios
              ├── /alerts
              ├── /notifications
              └── /watchlists
```

## Registry Pattern

Pluggable adapters and strategies register themselves at startup. The registry is a thread-safe singleton.

```go
// Register at init time
registry.Adapters.Register("binance", func(cfg Config) MarketAdapter { ... })
registry.Strategies.Register("ema_crossover", func(params JSON) Strategy { ... })

// Use at runtime
adapter := registry.Adapters.Get("binance")
strategy := registry.Strategies.Get("ema_crossover")
```

This allows adding new adapters/strategies without modifying core engine code.

## WebSocket Hub

The hub maintains a registry of connected clients and a map of channel subscriptions.

```
Client connects → Hub.Register(client)
Client sends subscribe msg → Hub.Subscribe(client, "ticks:BTC/USDT")
Backend emits event → Hub.Broadcast("ticks:BTC/USDT", payload)
Hub routes to subscribed clients via per-client buffered channel (256 msg)
Client disconnects → Hub.Unregister(client)
```

## Worker Pool

Long-running tasks (candle ingestion, backtest execution) are submitted as jobs to the pool. The pool maintains `WORKER_POOL_SIZE` goroutines. Each job is wrapped in a panic-recovering closure.

```go
pool.Submit(func() {
    runBacktest(backtestID)
})
```

## Frontend Data Flow

```
User action
  │
  ├── Zustand store (local state update)
  │
  ├── React Query mutation → Axios → /api/v1/... → backend
  │     └── on success → invalidate queries → refetch
  │
  └── WebSocket message (real-time updates)
        └── parsed in App.tsx WS handler → store.update()
```

## Data Persistence

- **MySQL**: persistent via Docker volume `mysql_data`. GORM AutoMigrate on every startup.
- **Redis**: persistent via `redis_data` volume with AOF + RDB. Used for: real-time tick cache, pub/sub, session data (future auth).

## Deployment Targets

| Target | Method |
|---|---|
| Development | `make up` (Docker Compose, hot-reload via Air + Vite HMR) |
| Production | Multi-stage Dockerfiles → static binary (Go) + Nginx (React) |
