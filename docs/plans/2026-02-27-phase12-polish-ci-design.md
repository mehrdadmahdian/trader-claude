# Phase 12 — Open Source Polish & CI: Design Document

**Date:** 2026-02-27
**Status:** Draft

---

## Overview

Phase 12 prepares the project for open-source release: comprehensive documentation (README, CONTRIBUTING, extension guides), GitHub CI/CD pipeline, seed data for demo, and a bonus paper trading mode that auto-executes monitor signals in a virtual portfolio.

---

## Decisions Made

| Topic | Decision |
|---|---|
| CI runner | GitHub Actions — `ubuntu-latest`, Docker Compose for integration tests |
| Lint tools | `golangci-lint` for Go, `eslint` + `tsc --noEmit` for frontend |
| Test strategy | Go: `go vet` + `go test ./...`; Frontend: `vitest run`; Docker: build check |
| Seed data | Makefile target `make seed` — Go script that calls adapters + inserts demo data |
| Seed symbols | Crypto: BTCUSDT, ETHUSDT, SOLUSDT (Binance); Stock: AAPL, MSFT, SPY (Yahoo) |
| Seed timeframe | 1 year daily candles for all 6 symbols |
| Paper trading | New `paper_trade` mode flag on Monitor — auto-creates transactions in a "Paper" portfolio |
| README banner | ASCII art or simple text banner — no external image hosting required |
| Architecture diagram | Mermaid in README — renders natively on GitHub |
| License | MIT (standard for open-source tools) |

---

## Section 1: Repository Documentation

### README.md

Structure:
```
# trader-claude

> Full-stack trading analytics platform built with Go + React

## Features
(bullet list of all 12 phases with screenshots)

## Architecture
(Mermaid diagram showing Docker services + data flow)

## Quick Start
1. Clone
2. cp .env.example .env
3. make up
4. Open http://localhost:5173

## Environment Variables
(table of all env vars with defaults)

## Adding a Market Adapter
(link to .claude/docs/adding-a-market.md)

## Adding a Strategy
(link to .claude/docs/adding-a-strategy.md)

## Screenshots
(4-6 screenshots of key pages)

## Contributing
(link to CONTRIBUTING.md)

## License
MIT
```

### CONTRIBUTING.md

```
## Development Setup
(Docker, make up, etc.)

## Code Style
- Go: gofmt + golangci-lint
- TypeScript: eslint + prettier
- Commit messages: conventional commits (feat/fix/docs/test/chore)

## Branch Naming
feature/phase-N-description
fix/issue-description

## Pull Request Process
1. Fork + branch
2. Implement + test
3. make backend-lint && make backend-test
4. make frontend-lint && make frontend-test
5. PR with description

## Testing
- Backend: go test ./...
- Frontend: vitest run
- All tests must pass before merge
```

### Extension Guides

`.claude/docs/adding-a-market.md`:
1. Create `backend/internal/adapter/{name}/adapter.go`
2. Implement `registry.MarketAdapter` interface
3. Register in `main.go`: `registry.Adapters.Register("name", New)`
4. Add to `.env.example` if API key needed
5. Full code example with FetchCandles + FetchSymbols

`.claude/docs/adding-a-strategy.md`:
1. Create `backend/internal/strategy/{name}/strategy.go`
2. Implement `registry.Strategy` interface
3. Register in `main.go`: `registry.Strategies.Register("name", New)`
4. Define `Params()` with ParamDefinition for auto-generated UI
5. Full code example with Init + OnCandle

---

## Section 2: GitHub CI/CD

### `.github/workflows/ci.yml`

```yaml
name: CI
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  backend:
    runs-on: ubuntu-latest
    services:
      mysql:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: test
          MYSQL_DATABASE: trader_test
        ports: ['3306:3306']
        options: --health-cmd="mysqladmin ping" --health-interval=10s --health-timeout=5s --health-retries=3
      redis:
        image: redis:7
        ports: ['6379:6379']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: go vet ./...
        working-directory: backend
      - run: golangci-lint run
        working-directory: backend
      - run: go test ./... -race -coverprofile=coverage.out
        working-directory: backend
        env:
          DB_HOST: localhost
          DB_PORT: 3306
          DB_USER: root
          DB_PASSWORD: test
          DB_NAME: trader_test
          REDIS_HOST: localhost
          REDIS_PORT: 6379

  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - run: npm ci
        working-directory: frontend
      - run: npx eslint .
        working-directory: frontend
      - run: npx tsc --noEmit
        working-directory: frontend
      - run: npx vitest run
        working-directory: frontend

  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker compose build
```

### Issue & PR Templates

`.github/ISSUE_TEMPLATE/bug_report.md`:
- Title, description, steps to reproduce, expected vs actual, environment

`.github/ISSUE_TEMPLATE/feature_request.md`:
- Title, description, use case, proposed solution

`.github/pull_request_template.md`:
- Description, type of change, testing checklist, screenshots

---

## Section 3: Seed Data

### `make seed` target

Runs `backend/cmd/seed/main.go`:

1. **Candle data** (6 symbols × 1 year daily):
   - Binance: BTCUSDT, ETHUSDT, SOLUSDT
   - Yahoo: AAPL, MSFT, SPY
   - Fetch via existing adapters, insert via DataService

2. **Demo portfolio** (3 positions):
   - BTCUSDT: 0.5 BTC @ $40,000
   - ETHUSDT: 5 ETH @ $2,200
   - AAPL: 10 shares @ $175

3. **Example monitor** (1 active):
   - EMA Crossover on BTCUSDT 1h

4. **Alert rules** (2):
   - Price above $100,000 for BTCUSDT
   - RSI oversold for ETHUSDT (custom condition)

5. **Sample news items** (5):
   - Realistic crypto/stock headlines with symbol tags

```go
// backend/cmd/seed/main.go
func main() {
    cfg := config.Load()
    db := connectDB(cfg)
    rdb := connectRedis(cfg)
    ds := adapter.NewDataService(db)

    seedCandles(ctx, ds)
    seedPortfolio(ctx, db)
    seedMonitor(ctx, db)
    seedAlerts(ctx, db)
    seedNews(ctx, db)

    log.Println("Seed complete!")
}
```

### Makefile target

```makefile
seed:
	docker compose exec backend go run ./cmd/seed/main.go
```

---

## Section 4: Paper Trading Mode (Bonus)

### Monitor Model Extension

Add field to `Monitor`:
```go
Mode string `gorm:"type:varchar(20);not null;default:'live_alert'" json:"mode"` // "live_alert" | "paper_trade"
PaperPortfolioID *int64 `gorm:"index" json:"paper_portfolio_id,omitempty"`
```

### Paper Trade Logic

In `monitor/poller.go`, after a signal is emitted:

```go
if mon.Mode == "paper_trade" && mon.PaperPortfolioID != nil {
    executePaperTrade(ctx, db, mon, sig)
}
```

`executePaperTrade`:
1. On LONG signal: create BUY transaction in the paper portfolio
2. On SHORT signal: create SELL transaction (close position)
3. Update portfolio current_value and current_cash
4. Create notification: "Paper trade: BUY 0.1 BTC @ $82,150"

### Frontend Changes

- Create Monitor modal: add "Mode" toggle (Live Alert / Paper Trade)
- When Paper Trade selected: show portfolio selector or auto-create "Paper: {monitor_name}" portfolio
- Monitor card: show paper trade stats (P&L since monitor start)
- Monitor detail: show paper trade transaction history

---

## Implementation Order

```
D1: README.md                                                    (no deps)
D2: CONTRIBUTING.md                                              (no deps, parallel)
D3: .claude/docs/adding-a-market.md                              (no deps, parallel)
D4: .claude/docs/adding-a-strategy.md                            (no deps, parallel)
C1: .github/ISSUE_TEMPLATE/bug_report.md + feature_request.md   (no deps)
C2: .github/pull_request_template.md                             (no deps)
C3: .github/workflows/ci.yml                                    (no deps)
S1: backend/cmd/seed/main.go — candle seeding                    (requires Phase 2 adapters)
S2: Seed portfolio, monitor, alerts, news                        (requires S1)
S3: Makefile target                                              (requires S1)
P1: Add Mode + PaperPortfolioID to Monitor model                 (requires Phase 8)
P2: executePaperTrade in poller.go                               (requires P1)
P3: Frontend: mode toggle in Create Monitor modal                (requires P1)
P4: Frontend: paper trade stats on Monitor card                  (requires P2)
```

---

## Testing Requirements

| Task | Tests |
|---|---|
| C3 | CI pipeline: manually verify by pushing to a test branch |
| S1 | Seed script: runs without error, candle count > 0 for all 6 symbols |
| S2 | Seed script: portfolio has 3 positions, 1 monitor, 2 alerts, 5 news items |
| P2 | Paper trade: LONG signal creates BUY transaction, SHORT creates SELL |
| P2 | Paper trade: portfolio value updates after transaction |
