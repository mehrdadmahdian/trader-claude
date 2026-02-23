# trader-claude — Claude Instructions

> Full-stack market backtesting, live monitoring, and research platform.
> Phase 1 complete. All further work builds on this scaffold.

---

## Quick Start

```bash
make setup       # First-time: copy .env, create dirs
make up          # Start all 4 Docker services
make health      # Verify backend: GET /health
make logs        # Tail all service logs
```

---

## Architecture at a Glance

```
trader-claude/
├── backend/          Go 1.24 + Fiber v2 + GORM + go-redis
├── frontend/         React 18 + TypeScript + Vite + Tailwind + shadcn/ui
├── docker/           MySQL init, Redis config, Nginx config
├── docs/             Architecture, API, DB, WS, Dev guide
├── .mcp.json         MCP server definitions
├── CLAUDE.md         ← this file
├── docker-compose.yml
└── Makefile
```

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend runtime | Go 1.24, Fiber v2 |
| ORM | GORM |
| DB | MySQL 8.0 (JSON columns, BIGINT PKs) |
| Cache | Redis 7 (AOF + RDB) |
| Hot reload | Air |
| Frontend | React 18, TypeScript, Vite 5 |
| Styling | TailwindCSS v3 (`darkMode: 'class'`) |
| Components | shadcn/ui (Radix primitives) |
| State | Zustand |
| Data fetching | React Query v5 |
| Charts | lightweight-charts, recharts |

---

## Backend Conventions

### File Layout
```
backend/
  cmd/server/main.go          # Entry point — DO NOT add business logic here
  internal/api/routes.go      # All HTTP + WS route registration
  internal/api/health.go      # Health handler (reference for handler pattern)
  internal/config/config.go   # Env-based config, all defaults here
  internal/models/models.go   # ALL GORM models — single source of truth
  internal/registry/          # Pluggable adapter + strategy registries
  internal/ws/hub.go          # WebSocket hub (singleton via sync.Once)
  internal/worker/pool.go     # Goroutine pool (size from config)
  migrations/001_init.sql     # Reference SQL — keep in sync with models.go
```

### Coding Rules
- **Never** add business logic to `main.go` — handlers live in `internal/api/`
- **Never** skip GORM model updates when changing `migrations/001_init.sql`
- Use `DECIMAL(20,8)` for prices, `DECIMAL(30,8)` for volumes — never `FLOAT`
- All MySQL columns use `JSON` (not JSONB — this is MySQL 8.0)
- Primary keys: `BIGINT AUTO_INCREMENT` (never `SERIAL`)
- Use `utf8mb4` charset everywhere (emoji support)
- Register new adapters in `internal/registry/registry.go` — never instantiate directly
- Add all new routes in `internal/api/routes.go` under `/api/v1/` prefix
- Fiber handler errors: always return `c.Status(xxx).JSON(fiber.Map{"error": "..."})`
- Worker jobs must have panic recovery — the pool provides this

### Startup Sequence (main.go)
Config → MySQL (retry 10x / 3s) → GORM AutoMigrate → Redis → WS Hub → Worker Pool → Fiber → Routes → Listen → Graceful Shutdown

### Key Interfaces
```go
// internal/registry/interfaces.go
type MarketAdapter interface { ... }
type Strategy interface { ... }
```

---

## Frontend Conventions

### File Layout
```
frontend/src/
  components/layout/    Layout.tsx, Sidebar.tsx, TopBar.tsx
  pages/                9 page components (Dashboard, Chart, Backtest, ...)
  stores/index.ts       6 Zustand stores — one file, all stores
  types/index.ts        ALL TypeScript interfaces — single source of truth
  api/client.ts         Axios instance with base URL from VITE_API_URL
  lib/utils.ts          Utility functions (cn, etc.)
```

### Coding Rules
- **All TS interfaces** go in `frontend/src/types/index.ts` — never inline in components
- **All Zustand stores** stay in `frontend/src/stores/index.ts`
- Theme: `localStorage` key `trader-theme`, Tailwind class `dark` on `<html>`
- API calls: use `api/client.ts` Axios instance — never raw `fetch`
- Data fetching: wrap all API calls in React Query `useQuery` / `useMutation`
- Shadcn components: run `npx shadcn-ui@latest add <component>` — never hand-code Radix
- Tailwind only — no inline styles, no CSS modules unless absolutely required
- Icons: `lucide-react` only — no other icon library

### Store Names
| Store | Purpose |
|---|---|
| `themeStore` | dark/light, persisted |
| `sidebarStore` | collapsed/expanded |
| `marketStore` | active symbol, timeframe, market, ticks |
| `backtestStore` | active + list, WS updates |
| `portfolioStore` | active + list, WS updates |
| `alertStore` | alerts list |
| `notificationStore` | notifications, unread count |

---

## API Conventions

- Base path: `/api/v1/`
- WebSocket: `GET /ws`
- Health: `GET /health` → `{status, db, redis, version}`
- All responses: `Content-Type: application/json`
- Error format: `{"error": "message"}`
- See `docs/api.md` for full route reference

---

## Database Conventions

- Engine: MySQL 8.0
- Charset: `utf8mb4_unicode_ci`
- PKs: `BIGINT AUTO_INCREMENT`
- Prices: `DECIMAL(20,8)`
- Volumes: `DECIMAL(30,8)`
- JSON fields: `JSON` type (MySQL native)
- Candles index: `(symbol, market, timeframe, timestamp)` composite
- See `docs/database.md` for full schema

---

## WebSocket Conventions

- Message types: `tick`, `candle`, `signal`, `alert`, `notification`, `error`, `ping`, `pong`
- Subscriptions: `ticks:{symbol}`, `candles:{symbol}:{timeframe}`, `alerts:{portfolio_id}`
- Client send buffer: 256 messages
- See `docs/websocket.md` for protocol spec

---

## Environment Variables

Managed via `.env` (copy from `.env.example`). Key vars:

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` | `development` | |
| `BACKEND_PORT` | `8080` | |
| `FRONTEND_PORT` | `5173` | |
| `DB_HOST` | `localhost` | `mysql` inside Docker |
| `REDIS_HOST` | `localhost` | `redis` inside Docker |
| `WORKER_POOL_SIZE` | `10` | |
| `JWT_SECRET` | — | **Required in prod** |
| `CORS_ORIGINS` | `http://localhost:5173` | |
| `VITE_API_URL` | `http://localhost:8080` | |
| `VITE_WS_URL` | `ws://localhost:8080` | |

---

## Makefile Reference

```bash
# Docker
make up / down / down-v / logs / restart / up-build

# Dev
make backend-shell    # Go container shell
make frontend-shell   # React container shell
make backend-test     # go test ./...
make backend-lint     # golangci-lint
make frontend-lint    # eslint

# DB
make migrate          # run migrations
make db-shell         # MySQL CLI (trader user)
make db-root          # MySQL CLI (root)
```

---

## Phase Roadmap

| Phase | Status | Description |
|---|---|---|
| 1 — Scaffold | **COMPLETE** | Docker, stub routes, all models, frontend shell |
| 2 — Data | Planned | Binance/Alpaca adapters, candle ingestion, live charts |
| 3 — Backtest | Planned | Strategy engine, backtest runner, results UI |
| 4 — Live | Planned | Paper/live trading, portfolio tracking |
| 5 — Auth | Planned | JWT auth, user accounts |
| 6 — News | Planned | News feed integration |

---

## Docs

- `.claude/docs/architecture.md` — System design and component interactions
- `.claude/docs/api.md` — Full HTTP API reference
- `.claude/docs/database.md` — Schema, indexes, conventions
- `.claude/docs/websocket.md` — WS protocol and message types
- `.claude/docs/development.md` — Dev workflow, debugging, tooling

---

## Important Notes

- Do not run `docker compose down -v` without user confirmation — it drops all data volumes
- Do not commit `.env` — only `.env.example` is tracked
- Do not use `float32`/`float64` for financial amounts — use `DECIMAL` in DB, `string`/`big.Float` in Go
- Do not add new dependencies without updating both `go.mod` (backend) and `package.json` (frontend)
- Always run `make backend-fmt` before committing Go code
- Always run `make frontend-fmt` before committing TypeScript code
