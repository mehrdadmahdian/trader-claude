# Development Guide

## Prerequisites

- Docker Desktop 4.x+
- Go 1.24+ (for local backend dev without Docker)
- Node.js 20+ (for local frontend dev without Docker)
- `make` (GNU Make)

---

## First-Time Setup

```bash
# 1. Clone and enter project
git clone <repo> trader-claude && cd trader-claude

# 2. Copy environment file
make setup
# or manually: cp .env.example .env

# 3. Start all services
make up

# 4. Verify health
make health
# Expected: {"status":"ok","db":"ok","redis":"ok","version":"0.1.0"}
```

---

## Daily Development Workflow

```bash
make up           # Start services (MySQL, Redis, Go, React)
make logs         # Tail all logs
make health       # Quick health check

# After code changes:
# Backend (Go): Air auto-reloads on file save — no action needed
# Frontend (React): Vite HMR updates browser instantly — no action needed

make down         # Stop services (data preserved in volumes)
make down-v       # Stop AND delete all data — use with caution
```

---

## Running Without Docker

Sometimes faster for focused backend/frontend work:

```bash
# Backend only (requires MySQL + Redis running locally or via Docker)
make dev-backend
# Uses Air hot-reload: backend/.air.toml

# Frontend only (proxies /api/* to backend via vite.config.ts)
make dev-frontend
```

---

## Backend Development

### Adding a New API Handler

1. Create handler function in `backend/internal/api/` (new file or existing)
2. Register route in `backend/internal/api/routes.go`
3. Use the health handler as reference: `backend/internal/api/health.go`

```go
// Pattern
func HandleThing(db *gorm.DB) fiber.Handler {
    return func(c *fiber.Ctx) error {
        // ...
        return c.Status(200).JSON(result)
        // or
        return c.Status(400).JSON(fiber.Map{"error": "bad request"})
    }
}
```

### Adding a New Model

1. Add struct to `backend/internal/models/models.go`
2. Add to `AutoMigrate` call in `main.go`
3. Update `backend/migrations/001_init.sql` to match
4. Restart backend container (Air will reload, AutoMigrate will create table)

### Adding a New Market Adapter

1. Create `backend/internal/adapters/{name}/adapter.go`
2. Implement `registry.MarketAdapter` interface from `backend/internal/registry/interfaces.go`
3. Register in `main.go`:
   ```go
   registry.Adapters.Register("binance", binance.New)
   ```

### Adding a New Strategy

1. Create `backend/internal/strategies/{name}/strategy.go`
2. Implement `registry.Strategy` interface
3. Register in `main.go`:
   ```go
   registry.Strategies.Register("ema_crossover", emacrossover.New)
   ```

### Running Tests

```bash
make backend-test
# or directly:
docker exec trader_backend go test ./...
```

### Code Quality

```bash
make backend-fmt    # gofmt
make backend-lint   # golangci-lint (must pass before PR)
```

---

## Frontend Development

### Adding a New Page

1. Create `frontend/src/pages/YourPage.tsx`
2. Add route in `frontend/src/App.tsx`
3. Add nav item in `frontend/src/components/layout/Sidebar.tsx`

### Adding a New Zustand Store

Add to `frontend/src/stores/index.ts` — keep all stores in one file.

```typescript
interface YourState {
  items: Item[]
  setItems: (items: Item[]) => void
}

export const useYourStore = create<YourState>((set) => ({
  items: [],
  setItems: (items) => set({ items }),
}))
```

### Adding a New TypeScript Type

Add to `frontend/src/types/index.ts` — keep all types in one file.

### Using React Query

```typescript
// Query (read)
const { data, isLoading, error } = useQuery({
  queryKey: ['backtests'],
  queryFn: () => api.get('/api/v1/backtests').then(r => r.data),
})

// Mutation (write)
const mutation = useMutation({
  mutationFn: (params) => api.post('/api/v1/backtests', params),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['backtests'] }),
})
```

### Adding a shadcn/ui Component

```bash
docker exec -it trader_frontend npx shadcn-ui@latest add button
# or locally:
cd frontend && npx shadcn-ui@latest add button
```

### Code Quality

```bash
make frontend-lint  # eslint (must pass)
make frontend-fmt   # prettier
```

---

## Database Operations

```bash
make db-shell       # MySQL CLI as trader user
make db-root        # MySQL CLI as root

# Inside MySQL shell:
SHOW TABLES;
DESCRIBE candles;
SELECT COUNT(*) FROM candles WHERE symbol='BTC/USDT' AND timeframe='1h';
```

---

## Logs and Debugging

```bash
make logs               # All services
make logs-backend       # Backend only (most useful)
make logs-frontend      # Frontend only

# Individual containers:
docker logs trader_backend -f
docker logs trader_mysql -f
docker logs trader_redis -f
docker logs trader_frontend -f
```

### Backend Debug Tips
- Check `APP_ENV=development` — enables verbose Fiber logging
- Redis connectivity: `docker exec trader_redis redis-cli ping`
- MySQL connectivity: `make db-shell`
- WebSocket: Use browser DevTools → Network → WS tab

---

## Environment Variables

Edit `.env` (never commit this file). Key variables for development:

```env
APP_ENV=development          # enables debug logging
LOG_LEVEL=debug
WORKER_POOL_SIZE=10
JWT_SECRET=dev-secret-change-in-prod
CORS_ORIGINS=http://localhost:6061
VITE_API_URL=http://localhost:6060
VITE_WS_URL=ws://localhost:6060
```

---

## Rebuilding After Dependency Changes

```bash
# Go dependency added (go.mod changed):
make up-build

# npm package added (package.json changed):
make up-build
# or just rebuild frontend:
docker compose build frontend && docker compose up -d frontend
```

---

## Common Issues

| Issue | Fix |
|---|---|
| Backend won't start (DB connection refused) | MySQL not healthy yet — wait 10s and retry |
| Frontend shows blank page | Check `VITE_API_URL` in `.env` matches running backend |
| CORS error in browser | Check `CORS_ORIGINS` in `.env` includes frontend URL |
| MySQL data looks wrong | Run `make db-shell` and inspect tables |
| Redis not persisting | Check `docker/redis/redis.conf` AOF settings |
| Air not reloading | Check `backend/.air.toml` for watched directories |
