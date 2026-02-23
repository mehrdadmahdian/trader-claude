# Database

**Engine:** MySQL 8.0
**Charset:** `utf8mb4_unicode_ci`
**Source of truth:** `backend/internal/models/models.go` (GORM) + `backend/migrations/001_init.sql`

> When you change `models.go`, always update `001_init.sql` to match.

---

## Tables

### symbols
Tradeable assets.

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| symbol | VARCHAR(20) NOT NULL | e.g. `BTC/USDT` |
| name | VARCHAR(100) | e.g. `Bitcoin` |
| market | ENUM | `crypto`, `stock`, `forex` |
| exchange | VARCHAR(50) | e.g. `binance` |
| base_currency | VARCHAR(10) | `BTC` |
| quote_currency | VARCHAR(10) | `USDT` |
| active | TINYINT(1) DEFAULT 1 | |
| metadata | JSON | |
| created_at | DATETIME | |
| updated_at | DATETIME | |

**Indexes:** `UNIQUE (symbol, market, exchange)`

---

### candles
OHLCV historical data — the largest table.

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| symbol | VARCHAR(20) NOT NULL | |
| market | VARCHAR(20) NOT NULL | |
| timeframe | VARCHAR(10) NOT NULL | `1m`, `5m`, `15m`, `1h`, `4h`, `1d` |
| timestamp | BIGINT NOT NULL | Unix seconds |
| open | DECIMAL(20,8) | |
| high | DECIMAL(20,8) | |
| low | DECIMAL(20,8) | |
| close | DECIMAL(20,8) | |
| volume | DECIMAL(30,8) | |
| created_at | DATETIME | |

**Indexes:**
- `UNIQUE (symbol, market, timeframe, timestamp)` — prevents duplicate candles
- Composite index on `(symbol, market, timeframe, timestamp)` — primary query path

**Note:** Never use `FLOAT` for OHLCV — always `DECIMAL(20,8)` / `DECIMAL(30,8)`.

---

### strategy_defs
Registered strategy definitions.

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| name | VARCHAR(100) NOT NULL | Human name |
| key | VARCHAR(100) UNIQUE | Registry key (e.g. `ema_crossover`) |
| description | TEXT | |
| params_schema | JSON | JSON Schema for params UI |
| version | VARCHAR(20) | |
| created_at | DATETIME | |
| updated_at | DATETIME | |

---

### backtests
Backtest run records.

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| strategy_id | BIGINT FK → strategy_defs | |
| symbol | VARCHAR(20) | |
| market | VARCHAR(20) | |
| timeframe | VARCHAR(10) | |
| start_date | DATETIME | |
| end_date | DATETIME | |
| initial_capital | DECIMAL(20,8) | |
| status | ENUM | `pending`, `running`, `completed`, `failed` |
| params | JSON | Strategy params used |
| metrics | JSON | Computed result metrics |
| equity_curve | JSON | Array of `{ts, equity}` |
| error_message | TEXT | Set on `failed` |
| created_at | DATETIME | |
| updated_at | DATETIME | |

---

### trades
Individual trade records (linked to backtest OR portfolio, not both).

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| backtest_id | BIGINT FK nullable | |
| portfolio_id | BIGINT FK nullable | |
| symbol | VARCHAR(20) | |
| market | VARCHAR(20) | |
| side | ENUM | `buy`, `sell` |
| entry_price | DECIMAL(20,8) | |
| exit_price | DECIMAL(20,8) nullable | |
| quantity | DECIMAL(30,8) | |
| pnl | DECIMAL(20,8) nullable | |
| pnl_pct | DECIMAL(10,4) nullable | |
| entry_time | DATETIME | |
| exit_time | DATETIME nullable | |
| metadata | JSON | |
| created_at | DATETIME | |

**Indexes:** `(backtest_id)`, `(portfolio_id)`, `(symbol, market)`

---

### portfolios
Paper or live trading portfolios.

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| name | VARCHAR(100) NOT NULL | |
| type | ENUM | `paper`, `live` |
| initial_capital | DECIMAL(20,8) | |
| current_value | DECIMAL(20,8) | |
| currency | VARCHAR(10) DEFAULT 'USDT' | |
| active | TINYINT(1) DEFAULT 1 | |
| metadata | JSON | |
| created_at | DATETIME | |
| updated_at | DATETIME | |

---

### alerts
Price/indicator alert definitions.

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| symbol | VARCHAR(20) NOT NULL | |
| market | VARCHAR(20) | |
| type | VARCHAR(50) | e.g. `price_above`, `price_below` |
| threshold | DECIMAL(20,8) | |
| message | TEXT | |
| active | TINYINT(1) DEFAULT 1 | |
| triggered_at | DATETIME nullable | |
| metadata | JSON | |
| created_at | DATETIME | |
| updated_at | DATETIME | |

---

### notifications
System notifications (alert fires, backtest complete, etc.).

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| type | VARCHAR(50) | e.g. `alert_triggered`, `backtest_done` |
| title | VARCHAR(200) | |
| body | TEXT | |
| read | TINYINT(1) DEFAULT 0 | |
| metadata | JSON | |
| created_at | DATETIME | |

---

### watch_lists
User-created collections of symbols.

| Column | Type | Notes |
|---|---|---|
| id | BIGINT AUTO_INCREMENT PK | |
| name | VARCHAR(100) NOT NULL | |
| symbols | JSON | Array of symbol strings |
| created_at | DATETIME | |
| updated_at | DATETIME | |

---

## Migrations

Schema is managed in two ways:

1. **Development:** GORM `AutoMigrate` runs on every backend startup — creates/alters tables to match models.
2. **Reference:** `backend/migrations/001_init.sql` — full DDL, kept in sync manually.

For production, use `001_init.sql` directly and manage schema changes with numbered migration files.

---

## Common Queries

```sql
-- Latest candles for a symbol/timeframe
SELECT * FROM candles
WHERE symbol = 'BTC/USDT' AND market = 'crypto' AND timeframe = '1h'
ORDER BY timestamp DESC
LIMIT 500;

-- Unread notifications
SELECT * FROM notifications WHERE read = 0 ORDER BY created_at DESC;

-- Active alerts
SELECT * FROM alerts WHERE active = 1;

-- Backtest results for a strategy
SELECT * FROM backtests WHERE strategy_id = 1 AND status = 'completed'
ORDER BY created_at DESC;
```
