# trader-claude — Bring-Up & Feature Testing Guide

> **Scope:** Every feature implemented as of March 2026 (Phase 1 scaffold + Bloomberg Phase A + Auth + Backtest engine + Analytics + Portfolio + Monitor + AI + Admin + Social + Replay + Indicators + Workspaces)

---

## 1. Prerequisites

| Requirement | Version |
|---|---|
| Docker Desktop | ≥ 24 |
| Docker Compose | v2 plugin (bundled with Docker Desktop) |
| `make` | any |
| `curl` | any (for API tests) |
| `wscat` | optional, for WebSocket tests (`npm i -g wscat`) |

---

## 2. First-Time Setup

```bash
# 1. Clone / enter the repo
cd trader-claude

# 2. Copy env file (only needed once)
make setup

# 3. Open .env and set at minimum:
#   JWT_SECRET=any-long-random-string
#   VITE_API_URL=http://localhost:8080
#   VITE_WS_URL=ws://localhost:8080
```

---

## 3. Starting All Services

```bash
make up
```

This starts 4 containers: `mysql`, `redis`, `backend` (Go/Fiber + Air), `frontend` (Vite).

Wait ~15 seconds for MySQL to initialise and the backend to connect.

Verify everything is running:

```bash
make health
# Expected: {"status":"ok","db":"ok","redis":"ok","version":"0.1.0"}

docker compose ps
# All 4 containers should show "Up"
```

Frontend is at: **http://localhost:5173**
Backend API is at: **http://localhost:8080**

---

## 4. Stopping / Cleanup

```bash
make down          # stop containers (keeps data volumes)
make down-v        # WARNING: also wipes MySQL + Redis data — confirm with user first
make logs          # tail all container logs
make restart       # down + up
```

---

## 5. Feature Test Checklist

Replace `TOKEN` below with the access token received from login. Set it once:

```bash
export TOKEN="<your_access_token>"
```

---

### 5.1 Health Check

```bash
curl http://localhost:8080/health
```

Expected:
```json
{"status":"ok","db":"ok","redis":"ok","version":"0.1.0"}
```

---

### 5.2 Auth

#### Register
```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Password1!","display_name":"Tester"}' | jq .
```
Expected: `201` with `user` + `access_token`.

#### Login
```bash
curl -s -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Password1!"}' | jq .
```
Expected: `200` with `user` + `access_token`. Cookie `refresh_token` set (httpOnly).

Save the token:
```bash
export TOKEN="<access_token from above>"
```

#### Get Profile
```bash
curl -s http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: user object.

#### Update Profile
```bash
curl -s -X PUT http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"display_name":"Updated Tester"}' | jq .
```

#### Refresh Token
```bash
curl -s -b cookies.txt -X POST http://localhost:8080/api/v1/auth/refresh | jq .
```
Expected: new `access_token`.

#### Logout
```bash
curl -s -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: `{"success":true}`.

#### Error Cases
- Register same email twice → `409 email already registered`
- Login with wrong password → `401 invalid email or password`
- Hit `/auth/login` 6 times in 1 minute → `429 too many login attempts`

---

### 5.3 Markets & Candles

#### List Adapters (registered market data sources)
```bash
curl -s http://localhost:8080/api/v1/markets \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### List Symbols for an Adapter
```bash
# Replace <adapterID> with one returned by the above call
curl -s "http://localhost:8080/api/v1/markets/<adapterID>/symbols" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### List Available Timeframes
```bash
curl -s http://localhost:8080/api/v1/candles/timeframes \
  -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: array like `["1m","5m","15m","1h","4h","1d"]`.

#### Query Candles
```bash
curl -s "http://localhost:8080/api/v1/candles?symbol=BTC/USDT&market=crypto&timeframe=1h&limit=10" \
  -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: array of OHLCV objects (empty if no data ingested yet).

---

### 5.4 Indicators

#### List Indicators
```bash
curl -s http://localhost:8080/api/v1/indicators \
  -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: array of indicator definitions (name, params).

#### Calculate an Indicator
```bash
curl -s -X POST http://localhost:8080/api/v1/indicators/calculate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "indicator": "EMA",
    "params": {"period": 14},
    "data": [100,102,101,103,105,104,106,108,107,109,110,111,112,113,114,115]
  }' | jq .
```
Expected: array of calculated values.

---

### 5.5 Strategies

#### List Strategies
```bash
curl -s http://localhost:8080/api/v1/strategies \
  -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: array of registered strategy definitions.

#### Get Strategy by ID
```bash
curl -s http://localhost:8080/api/v1/strategies/1 \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

### 5.6 Backtests

#### Run a Backtest
```bash
curl -s -X POST http://localhost:8080/api/v1/backtest/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "adapter_id": "<adapterID>",
    "symbol": "BTC/USDT",
    "market": "crypto",
    "timeframe": "1h",
    "strategy_name": "<strategy_name>",
    "start_date": "2023-01-01T00:00:00Z",
    "end_date": "2023-06-01T00:00:00Z",
    "initial_capital": "10000",
    "params": {}
  }' | jq .
```
Expected: `201` with backtest run object and `status: "pending"`.

Save the run ID:
```bash
export RUN_ID=<id from above>
```

#### List Backtest Runs
```bash
curl -s http://localhost:8080/api/v1/backtest/runs \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Get a Specific Run
```bash
curl -s http://localhost:8080/api/v1/backtest/runs/$RUN_ID \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Watch Backtest Progress via WebSocket
```bash
wscat -c "ws://localhost:8080/ws/backtest/$RUN_ID/progress?token=$TOKEN"
```
Expected: progress events streamed until completion.

#### Delete a Run
```bash
curl -s -X DELETE http://localhost:8080/api/v1/backtest/runs/$RUN_ID \
  -H "Authorization: Bearer $TOKEN"
```
Expected: `204`.

---

### 5.7 Analytics (requires a completed backtest run)

#### Param Heatmap
```bash
curl -s "http://localhost:8080/api/v1/backtest/runs/$RUN_ID/param-heatmap" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Monte Carlo Simulation (async job)
```bash
curl -s -X POST "http://localhost:8080/api/v1/backtest/runs/$RUN_ID/monte-carlo" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"simulations": 100}' | jq .
```
Returns `job_id`. Poll the job:
```bash
export JOB_ID=<job_id>
curl -s "http://localhost:8080/api/v1/analytics/jobs/$JOB_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Walk-Forward Analysis
```bash
curl -s "http://localhost:8080/api/v1/backtest/runs/$RUN_ID/walk-forward" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Compare Runs
```bash
curl -s -X POST http://localhost:8080/api/v1/backtest/compare \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"run_ids": ['$RUN_ID']}' | jq .
```

---

### 5.8 Replay (requires a completed backtest run)

#### Create a Replay Session
```bash
curl -s -X POST "http://localhost:8080/api/v1/backtest/runs/$RUN_ID/replay" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"speed": 1}' | jq .
```
Save `replay_id`.

#### Connect to Replay WebSocket
```bash
export REPLAY_ID=<replay_id>
wscat -c "ws://localhost:8080/ws/replay/$REPLAY_ID?token=$TOKEN"
```
Expected: candle + signal events replayed in sequence.

#### Create a Bookmark
```bash
curl -s -X POST http://localhost:8080/api/v1/replay/bookmarks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"replay_id": '$REPLAY_ID', "label": "interesting moment", "timestamp": 1672531200}' | jq .
```

#### List / Get / Delete Bookmarks
```bash
curl -s http://localhost:8080/api/v1/replay/bookmarks -H "Authorization: Bearer $TOKEN" | jq .
curl -s http://localhost:8080/api/v1/replay/bookmarks/1 -H "Authorization: Bearer $TOKEN" | jq .
curl -s -X DELETE http://localhost:8080/api/v1/replay/bookmarks/1 -H "Authorization: Bearer $TOKEN"
```

---

### 5.9 Portfolios

#### Create Portfolio
```bash
curl -s -X POST http://localhost:8080/api/v1/portfolios \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Paper Portfolio","type":"paper","initial_capital":"10000","currency":"USDT"}' | jq .
```
Save `portfolio_id`.

```bash
export PID=<portfolio_id>
```

#### List / Get Portfolio
```bash
curl -s http://localhost:8080/api/v1/portfolios -H "Authorization: Bearer $TOKEN" | jq .
curl -s http://localhost:8080/api/v1/portfolios/$PID -H "Authorization: Bearer $TOKEN" | jq .
```

#### Get Portfolio Summary
```bash
curl -s http://localhost:8080/api/v1/portfolios/$PID/summary \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Add Position
```bash
curl -s -X POST http://localhost:8080/api/v1/portfolios/$PID/positions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC/USDT","quantity":"0.5","avg_cost":"42000","market":"crypto"}' | jq .
```
Save `position_id`.

#### Update / Delete Position
```bash
export POS_ID=<position_id>
curl -s -X PUT http://localhost:8080/api/v1/portfolios/$PID/positions/$POS_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"quantity":"1.0"}' | jq .

curl -s -X DELETE http://localhost:8080/api/v1/portfolios/$PID/positions/$POS_ID \
  -H "Authorization: Bearer $TOKEN"
```

#### Add Transaction
```bash
curl -s -X POST http://localhost:8080/api/v1/portfolios/$PID/transactions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"buy","symbol":"BTC/USDT","quantity":"0.1","price":"43000","fee":"10"}' | jq .
```

#### List Transactions / Equity Curve
```bash
curl -s http://localhost:8080/api/v1/portfolios/$PID/transactions -H "Authorization: Bearer $TOKEN" | jq .
curl -s http://localhost:8080/api/v1/portfolios/$PID/equity-curve -H "Authorization: Bearer $TOKEN" | jq .
```

---

### 5.10 Alerts

#### Create Alert
```bash
curl -s -X POST http://localhost:8080/api/v1/alerts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC/USDT","market":"crypto","type":"price_above","threshold":"50000","message":"BTC above 50k!"}' | jq .
```
Save `alert_id`.

#### List Alerts
```bash
curl -s http://localhost:8080/api/v1/alerts -H "Authorization: Bearer $TOKEN" | jq .
curl -s "http://localhost:8080/api/v1/alerts?active=true" -H "Authorization: Bearer $TOKEN" | jq .
```

#### Toggle Alert (activate/deactivate)
```bash
export AID=<alert_id>
curl -s -X PATCH http://localhost:8080/api/v1/alerts/$AID/toggle \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Delete Alert
```bash
curl -s -X DELETE http://localhost:8080/api/v1/alerts/$AID \
  -H "Authorization: Bearer $TOKEN"
```

---

### 5.11 Notifications

#### List Notifications
```bash
curl -s http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $TOKEN" | jq .

curl -s "http://localhost:8080/api/v1/notifications?unread=true" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Get Unread Count
```bash
curl -s http://localhost:8080/api/v1/notifications/unread-count \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Mark Read
```bash
curl -s -X PATCH http://localhost:8080/api/v1/notifications/1/read \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Mark All Read
```bash
curl -s -X POST http://localhost:8080/api/v1/notifications/read-all \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Notifications WebSocket
```bash
wscat -c "ws://localhost:8080/ws/notifications?token=$TOKEN"
```
Expected: real-time notification push events.

---

### 5.12 Monitors (Live Signal Runner)

#### Create Monitor
```bash
curl -s -X POST http://localhost:8080/api/v1/monitors \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "BTC EMA Monitor",
    "adapter_id": "<adapterID>",
    "symbol": "BTC/USDT",
    "market": "crypto",
    "timeframe": "1h",
    "strategy_name": "<strategy_name>",
    "params": {},
    "notify_in_app": true
  }' | jq .
```
Save `monitor_id`.

```bash
export MID=<monitor_id>
```

#### List / Get Monitor
```bash
curl -s http://localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN" | jq .
curl -s http://localhost:8080/api/v1/monitors/$MID -H "Authorization: Bearer $TOKEN" | jq .
```

#### Toggle Monitor (start/stop)
```bash
curl -s -X PATCH http://localhost:8080/api/v1/monitors/$MID/toggle \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Get Monitor Signals (historical)
```bash
curl -s http://localhost:8080/api/v1/monitors/$MID/signals \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Live Signals WebSocket
```bash
wscat -c "ws://localhost:8080/ws/monitors/signals?token=$TOKEN"
# Then send subscription message:
# {"type":"subscribe","monitor_id":<MID>}
```

---

### 5.13 News

#### List News
```bash
curl -s http://localhost:8080/api/v1/news \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### News by Symbol
```bash
curl -s "http://localhost:8080/api/v1/news/symbols/BTC%2FUSDT" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

### 5.14 Workspaces (Bloomberg Terminal)

#### Create Workspace
```bash
curl -s -X POST http://localhost:8080/api/v1/workspaces \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Workspace",
    "layout": [{"i":"panel-1","x":0,"y":0,"w":6,"h":4}],
    "panel_states": {"panel-1":{"type":"chart","symbol":"BTC/USDT"}}
  }' | jq .
```
Save `workspace_id`.

```bash
export WID=<workspace_id>
```

#### List / Get Workspace
```bash
curl -s http://localhost:8080/api/v1/workspaces -H "Authorization: Bearer $TOKEN" | jq .
curl -s http://localhost:8080/api/v1/workspaces/$WID -H "Authorization: Bearer $TOKEN" | jq .
```

#### Update Workspace
```bash
curl -s -X PUT http://localhost:8080/api/v1/workspaces/$WID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Renamed Workspace"}' | jq .
```

#### Delete Workspace
```bash
curl -s -X DELETE http://localhost:8080/api/v1/workspaces/$WID \
  -H "Authorization: Bearer $TOKEN"
```

---

### 5.15 Settings

#### Get Notification Settings
```bash
curl -s http://localhost:8080/api/v1/settings/notifications \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Save Notification Settings
```bash
curl -s -X POST http://localhost:8080/api/v1/settings/notifications \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email_enabled":false,"in_app_enabled":true}' | jq .
```

#### Test Notification Delivery
```bash
curl -s -X POST http://localhost:8080/api/v1/settings/notifications/test \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

### 5.16 AI Chat

#### Get AI Settings
```bash
curl -s http://localhost:8080/api/v1/settings/ai \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Save AI Settings (API key, model, etc.)
```bash
curl -s -X POST http://localhost:8080/api/v1/settings/ai \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"provider":"anthropic","api_key":"sk-ant-...","model":"claude-3-5-sonnet-20241022"}' | jq .
```

#### Test AI Connection
```bash
curl -s -X POST http://localhost:8080/api/v1/settings/ai/test \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Send Chat Message
```bash
curl -s -X POST http://localhost:8080/api/v1/ai/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message":"What is EMA crossover strategy?","context":{}}' | jq .
```

---

### 5.17 Social Cards

#### Generate Backtest Share Card
```bash
curl -s -X POST "http://localhost:8080/api/v1/social/backtest-card/$RUN_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Generate Signal Share Card
```bash
curl -s -X POST "http://localhost:8080/api/v1/social/signal-card/1" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

#### Send via Telegram (requires Telegram settings configured)
```bash
curl -s -X POST http://localhost:8080/api/v1/social/send-telegram \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message":"Test message","type":"backtest","ref_id":'$RUN_ID'}' | jq .
```

---

### 5.18 Admin (requires admin role user)

> First, promote a user to admin directly in MySQL or use a seed. Then re-login to get a token with `role: admin`.

#### List Users
```bash
curl -s http://localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

#### Toggle User (enable/disable)
```bash
curl -s -X PATCH "http://localhost:8080/api/v1/admin/users/2/toggle" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

#### Change User Role
```bash
curl -s -X PATCH "http://localhost:8080/api/v1/admin/users/2/role" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role":"admin"}' | jq .
```

#### Error Case: Non-admin gets 403
```bash
curl -s http://localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer $TOKEN"
# Expected: 403 forbidden
```

---

### 5.19 WebSocket Hub (Market Data)

```bash
wscat -c "ws://localhost:8080/ws?token=$TOKEN"
```

Once connected:
```json
// Subscribe to tick stream
{"type":"subscribe","channel":"ticks:BTC/USDT"}

// Subscribe to candles
{"type":"subscribe","channel":"candles:BTC/USDT:1h"}

// Ping/pong
{"type":"ping"}

// Unsubscribe
{"type":"unsubscribe","channel":"ticks:BTC/USDT"}
```
Expected: `{"type":"pong"}` on ping; tick/candle events if an adapter is feeding data.

---

## 6. Frontend UI Walkthrough

Open **http://localhost:5173** in a browser.

| URL | What to verify |
|---|---|
| `/login` | Login form works, redirects to `/` on success |
| `/register` | Registration form works, error on duplicate email |
| `/` (Dashboard) | Watchlist panel, news panel, alerts feed, portfolio summary render without errors |
| `/chart` | Symbol selector, timeframe selector, candlestick chart renders; indicator modal opens and indicator chips appear on chart |
| `/backtest` | Can run a backtest, progress bar updates via WS, results table and metrics appear on completion; Analytics tab shows heatmap/monte carlo/walk-forward |
| `/portfolio` | Create portfolio, add position, view equity curve chart, allocation donut |
| `/monitor` | Create monitor, toggle it on/off, signals list populates |
| `/alerts` | Create and delete alerts, toggle active state |
| `/news` | News articles list, click article for details |
| `/notifications` | Notification list, mark individual and all read, unread badge in sidebar |
| `/settings` | Notification settings form saves; AI settings saves; test connection button |
| `/terminal` | Bloomberg workspace opens; workspace tabs appear; panels can be added/dragged; layout persists after page reload |

---

## 7. Running Backend Tests

```bash
make backend-test
# Runs: go test ./...
```

Key test files:
- `backend/internal/api/backtest_test.go`
- `backend/internal/api/portfolio_test.go`
- `backend/internal/api/security_test.go`
- `backend/internal/api/monitor_handler_test.go`
- `backend/internal/api/replay_test.go`

---

## 8. Common Troubleshooting

| Symptom | Fix |
|---|---|
| `make health` returns `db: error` | MySQL is still starting — wait 10–20s and retry |
| `401 unauthorized` on all API calls | Token expired — re-login to get a new token |
| Frontend shows "Network Error" | Check `VITE_API_URL` in `.env` matches the backend port |
| `429 too many requests` on login | Wait 1 minute — rate limiter resets per minute |
| WebSocket connection refused | Token must be passed as `?token=<TOKEN>` query param |
| Container won't start | Run `make logs` to see error detail |
| Backend won't compile | Run `make backend-shell` then `go build ./...` inside the container |

---

## 9. Logs & Debugging

```bash
make logs                           # all services
docker compose logs backend         # Go server only
docker compose logs frontend        # Vite only
docker compose logs mysql           # DB only

make backend-shell                  # shell inside Go container
make db-shell                       # MySQL CLI (trader user)
make db-root                        # MySQL CLI (root)
```
