package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// JSON is a helper type for storing arbitrary JSON in MySQL JSON columns
type JSON map[string]interface{}

func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// JSONArray is a helper type for storing JSON arrays
type JSONArray []interface{}

func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// --- Candle ---

// Candle stores historical OHLCV data
type Candle struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Symbol    string         `gorm:"type:varchar(20);not null;index:idx_candle_lookup,priority:1" json:"symbol"`
	Market    string         `gorm:"type:varchar(20);not null;index:idx_candle_lookup,priority:2" json:"market"`
	Timeframe string         `gorm:"type:varchar(10);not null;index:idx_candle_lookup,priority:3" json:"timeframe"`
	Timestamp time.Time      `gorm:"not null;index:idx_candle_lookup,priority:4" json:"timestamp"`
	Open      float64        `gorm:"type:decimal(20,8);not null" json:"open"`
	High      float64        `gorm:"type:decimal(20,8);not null" json:"high"`
	Low       float64        `gorm:"type:decimal(20,8);not null" json:"low"`
	Close     float64        `gorm:"type:decimal(20,8);not null" json:"close"`
	Volume    float64        `gorm:"type:decimal(30,8);not null" json:"volume"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Candle) TableName() string { return "candles" }

// --- Symbol ---

// Symbol represents a tradeable asset
type Symbol struct {
	ID          int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Ticker      string         `gorm:"type:varchar(20);not null;uniqueIndex:idx_symbol_market" json:"ticker"`
	Market      string         `gorm:"type:varchar(20);not null;uniqueIndex:idx_symbol_market" json:"market"`
	BaseAsset   string         `gorm:"type:varchar(20);not null" json:"base_asset"`
	QuoteAsset  string         `gorm:"type:varchar(20)" json:"quote_asset"`
	Description string         `gorm:"type:varchar(255)" json:"description"`
	Active      bool           `gorm:"default:true" json:"active"`
	Metadata    JSON           `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Symbol) TableName() string { return "symbols" }

// --- Strategy (definition) ---

// StrategyDef stores a registered strategy definition
type StrategyDef struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	Description  string    `gorm:"type:text" json:"description"`
	ParamsSchema JSON      `gorm:"type:json" json:"params_schema"`
	Active       bool      `gorm:"default:true" json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (StrategyDef) TableName() string { return "strategy_defs" }

// --- Backtest ---

// BacktestStatus represents the lifecycle of a backtest run
type BacktestStatus string

const (
	BacktestStatusPending   BacktestStatus = "pending"
	BacktestStatusRunning   BacktestStatus = "running"
	BacktestStatusCompleted BacktestStatus = "completed"
	BacktestStatusFailed    BacktestStatus = "failed"
	BacktestStatusCancelled BacktestStatus = "cancelled"
)

// Backtest stores a backtest run
type Backtest struct {
	ID           int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string         `gorm:"type:varchar(200);not null" json:"name"`
	AdapterID    string         `gorm:"type:varchar(50);not null;default:''" json:"adapter_id"`
	StrategyName string         `gorm:"type:varchar(100);not null;index" json:"strategy_name"`
	Symbol       string         `gorm:"type:varchar(20);not null" json:"symbol"`
	Market       string         `gorm:"type:varchar(20);not null" json:"market"`
	Timeframe    string         `gorm:"type:varchar(10);not null" json:"timeframe"`
	StartDate    time.Time      `gorm:"not null" json:"start_date"`
	EndDate      time.Time      `gorm:"not null" json:"end_date"`
	Params       JSON           `gorm:"type:json" json:"params"`
	Status       BacktestStatus `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	Metrics      JSON           `gorm:"type:json" json:"metrics,omitempty"`
	EquityCurve  JSONArray      `gorm:"type:json" json:"equity_curve,omitempty"`
	ErrorMessage string         `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Backtest) TableName() string { return "backtests" }

// --- Trade ---

// TradeDirection is long or short
type TradeDirection string

const (
	TradeDirectionLong  TradeDirection = "long"
	TradeDirectionShort TradeDirection = "short"
)

// Trade stores individual trade records from backtest or live runs
type Trade struct {
	ID          int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	BacktestID  *int64         `gorm:"index" json:"backtest_id,omitempty"`
	PortfolioID *int64         `gorm:"index" json:"portfolio_id,omitempty"`
	Symbol      string         `gorm:"type:varchar(20);not null;index" json:"symbol"`
	Market      string         `gorm:"type:varchar(20);not null" json:"market"`
	Direction   TradeDirection `gorm:"type:varchar(10);not null" json:"direction"`
	EntryPrice  float64        `gorm:"type:decimal(20,8);not null" json:"entry_price"`
	ExitPrice   *float64       `gorm:"type:decimal(20,8)" json:"exit_price,omitempty"`
	Quantity    float64        `gorm:"type:decimal(20,8);not null" json:"quantity"`
	EntryTime   time.Time      `gorm:"not null" json:"entry_time"`
	ExitTime    *time.Time     `json:"exit_time,omitempty"`
	PnL         *float64       `gorm:"type:decimal(20,8)" json:"pnl,omitempty"`
	PnLPercent  *float64       `gorm:"type:decimal(10,4)" json:"pnl_percent,omitempty"`
	Fee         float64        `gorm:"type:decimal(20,8);default:0" json:"fee"`
	Metadata    JSON           `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func (Trade) TableName() string { return "trades" }

// --- Portfolio ---

// PortfolioType classifies how a portfolio is managed
type PortfolioType string

const (
	PortfolioTypeManual PortfolioType = "manual"
	PortfolioTypePaper  PortfolioType = "paper"
	PortfolioTypeLive   PortfolioType = "live"
)

// Portfolio represents a live trading, paper trading, or manual portfolio
type Portfolio struct {
	ID           int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string         `gorm:"type:varchar(200);not null" json:"name"`
	Description  string         `gorm:"type:text" json:"description"`
	Type         PortfolioType  `gorm:"type:varchar(20);not null;default:'manual'" json:"type"`
	Currency     string         `gorm:"type:varchar(10);not null;default:'USD'" json:"currency"`
	StrategyName string         `gorm:"type:varchar(100)" json:"strategy_name"`
	Symbol       string         `gorm:"type:varchar(20)" json:"symbol"`
	Market       string         `gorm:"type:varchar(20)" json:"market"`
	Timeframe    string         `gorm:"type:varchar(10)" json:"timeframe"`
	Params       JSON           `gorm:"type:json" json:"params"`
	IsLive       bool           `gorm:"default:false" json:"is_live"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	InitialCash  float64        `gorm:"type:decimal(20,8);not null" json:"initial_cash"`
	CurrentCash  float64        `gorm:"type:decimal(20,8)" json:"current_cash"`
	CurrentValue float64        `gorm:"type:decimal(20,8)" json:"current_value"`
	State        JSON           `gorm:"type:json" json:"state,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Portfolio) TableName() string { return "portfolios" }

// --- Position ---

// Position tracks an open holding within a portfolio
type Position struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	PortfolioID      int64     `gorm:"not null;index" json:"portfolio_id"`
	AdapterID        string    `gorm:"type:varchar(20);not null" json:"adapter_id"`
	Symbol           string    `gorm:"type:varchar(20);not null" json:"symbol"`
	Market           string    `gorm:"type:varchar(20);not null" json:"market"`
	Quantity         float64   `gorm:"type:decimal(30,8);not null" json:"quantity"`
	AvgCost          float64   `gorm:"type:decimal(20,8);not null" json:"avg_cost"`
	CurrentPrice     float64   `gorm:"type:decimal(20,8)" json:"current_price"`
	CurrentValue     float64   `gorm:"type:decimal(20,8)" json:"current_value"`
	UnrealizedPnL    float64   `gorm:"type:decimal(20,8)" json:"unrealized_pnl"`
	UnrealizedPnLPct float64   `gorm:"type:decimal(10,4)" json:"unrealized_pnl_pct"`
	OpenedAt         time.Time `gorm:"not null" json:"opened_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (Position) TableName() string { return "positions" }

// --- Transaction ---

// TransactionType classifies the kind of ledger entry
type TransactionType string

const (
	TransactionTypeBuy        TransactionType = "buy"
	TransactionTypeSell       TransactionType = "sell"
	TransactionTypeDeposit    TransactionType = "deposit"
	TransactionTypeWithdrawal TransactionType = "withdrawal"
)

// Transaction records every cash or asset movement within a portfolio
type Transaction struct {
	ID          int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	PortfolioID int64           `gorm:"not null;index" json:"portfolio_id"`
	PositionID  *int64          `gorm:"index" json:"position_id,omitempty"`
	Type        TransactionType `gorm:"type:varchar(20);not null" json:"type"`
	AdapterID   string          `gorm:"type:varchar(20)" json:"adapter_id"`
	Symbol      string          `gorm:"type:varchar(20)" json:"symbol"`
	Quantity    float64         `gorm:"type:decimal(30,8)" json:"quantity"`
	Price       float64         `gorm:"type:decimal(20,8);not null" json:"price"`
	Fee         float64         `gorm:"type:decimal(20,8);default:0" json:"fee"`
	Notes       string          `gorm:"type:text" json:"notes"`
	ExecutedAt  time.Time       `gorm:"not null" json:"executed_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (Transaction) TableName() string { return "transactions" }

// --- Alert ---

// AlertStatus is the lifecycle state of an alert
type AlertStatus string

const (
	AlertStatusActive    AlertStatus = "active"
	AlertStatusTriggered AlertStatus = "triggered"
	AlertStatusDisabled  AlertStatus = "disabled"
)

// AlertCondition is the type of trigger condition
type AlertCondition string

const (
	AlertConditionPriceAbove  AlertCondition = "price_above"
	AlertConditionPriceBelow  AlertCondition = "price_below"
	AlertConditionPriceChange AlertCondition = "price_change_pct"
	AlertConditionVolume      AlertCondition = "volume_spike"
	AlertConditionCustom      AlertCondition = "custom"
)

// Alert stores price/volume/custom alert definitions
type Alert struct {
	ID               int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string         `gorm:"type:varchar(200);not null" json:"name"`
	AdapterID        string         `gorm:"type:varchar(20);not null;default:'binance'" json:"adapter_id"`
	Symbol           string         `gorm:"type:varchar(20);not null;index" json:"symbol"`
	Market           string         `gorm:"type:varchar(20);not null" json:"market"`
	Condition        AlertCondition `gorm:"type:varchar(30);not null" json:"condition"`
	Threshold        float64        `gorm:"type:decimal(20,8);not null" json:"threshold"`
	BasePrice        float64        `gorm:"type:decimal(20,8);default:0" json:"base_price"`
	Status           AlertStatus    `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
	Message          string         `gorm:"type:text" json:"message"`
	RecurringEnabled bool           `gorm:"default:true" json:"recurring_enabled"`
	CooldownMinutes  int            `gorm:"default:60" json:"cooldown_minutes"`
	LastFiredAt      *time.Time     `json:"last_fired_at,omitempty"`
	TriggeredAt      *time.Time     `json:"triggered_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

func (Alert) TableName() string { return "alerts" }

// --- NewsItem ---

// NewsItem stores a de-duplicated RSS article
type NewsItem struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	URL         string    `gorm:"type:varchar(2048);uniqueIndex" json:"url"`
	Title       string    `gorm:"type:varchar(512);not null" json:"title"`
	Summary     string    `gorm:"type:text" json:"summary"`
	Source      string    `gorm:"type:varchar(64);not null;index" json:"source"`
	PublishedAt time.Time `gorm:"index" json:"published_at"`
	Symbols     JSONArray `gorm:"type:json" json:"symbols"`
	Sentiment   float64   `gorm:"type:decimal(4,3)" json:"sentiment"`
	FetchedAt   time.Time `json:"fetched_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (NewsItem) TableName() string { return "news_items" }

// --- Notification ---

// NotificationType identifies the kind of notification
type NotificationType string

const (
	NotificationTypeAlert    NotificationType = "alert"
	NotificationTypeTrade    NotificationType = "trade"
	NotificationTypeSystem   NotificationType = "system"
	NotificationTypeBacktest NotificationType = "backtest"
	NotificationTypeSignal   NotificationType = "signal"
)

// Notification stores system/alert notifications
type Notification struct {
	ID        int64            `gorm:"primaryKey;autoIncrement" json:"id"`
	Type      NotificationType `gorm:"type:varchar(30);not null;index" json:"type"`
	Title     string           `gorm:"type:varchar(255);not null" json:"title"`
	Body      string           `gorm:"type:text" json:"body"`
	Read      bool             `gorm:"default:false;index" json:"read"`
	Metadata  JSON             `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt time.Time        `gorm:"index" json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

// --- WatchList ---

// WatchList groups symbols a user wants to track
type WatchList struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string         `gorm:"type:varchar(100);not null" json:"name"`
	Symbols   JSON           `gorm:"type:json" json:"symbols"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (WatchList) TableName() string { return "watch_lists" }

// --- Monitor ---

// MonitorStatus is the lifecycle state of a monitor
type MonitorStatus string

const (
	MonitorStatusActive  MonitorStatus = "active"
	MonitorStatusPaused  MonitorStatus = "paused"
	MonitorStatusStopped MonitorStatus = "stopped"
)

// Monitor stores a live strategy-monitoring configuration
type Monitor struct {
	ID              int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string         `gorm:"type:varchar(200);not null" json:"name"`
	AdapterID       string         `gorm:"type:varchar(20);not null" json:"adapter_id"`
	Symbol          string         `gorm:"type:varchar(20);not null;index" json:"symbol"`
	Market          string         `gorm:"type:varchar(20);not null" json:"market"`
	Timeframe       string         `gorm:"type:varchar(10);not null" json:"timeframe"`
	StrategyName    string         `gorm:"type:varchar(100);not null;index" json:"strategy_name"`
	Params          JSON           `gorm:"type:json" json:"params"`
	Status          MonitorStatus  `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
	NotifyInApp     bool           `gorm:"default:true" json:"notify_in_app"`
	LastPolledAt    *time.Time     `json:"last_polled_at,omitempty"`
	LastSignalAt    *time.Time     `json:"last_signal_at,omitempty"`
	LastSignalDir   string         `gorm:"type:varchar(10)" json:"last_signal_dir"`
	LastSignalPrice float64        `gorm:"type:decimal(20,8)" json:"last_signal_price"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Monitor) TableName() string { return "monitors" }

// MonitorSignal stores a strategy signal emitted by a live monitor
type MonitorSignal struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	MonitorID int64     `gorm:"not null;index" json:"monitor_id"`
	Direction string    `gorm:"type:varchar(10);not null" json:"direction"` // "long","short","flat"
	Price     float64   `gorm:"type:decimal(20,8);not null" json:"price"`
	Strength  float64   `gorm:"type:decimal(5,4);not null" json:"strength"`
	Metadata  JSON      `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (MonitorSignal) TableName() string { return "monitor_signals" }

// --- ReplayBookmark ---

// ReplayBookmark stores an annotated moment in a replay session for future research.
type ReplayBookmark struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        int64     `gorm:"index;default:1" json:"user_id"`
	BacktestRunID int64     `gorm:"index;not null" json:"backtest_run_id"`
	CandleIndex   int       `gorm:"not null" json:"candle_index"`
	Label         string    `gorm:"type:varchar(255)" json:"label"`
	Note          string    `gorm:"type:text" json:"note"`
	ChartSnapshot string    `gorm:"type:longtext" json:"chart_snapshot"` // base64 PNG
	CreatedAt     time.Time `json:"created_at"`
}

func (ReplayBookmark) TableName() string { return "replay_bookmarks" }

// --- Setting ---

// Setting stores per-user configuration as key-value JSON pairs.
type Setting struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"default:1;uniqueIndex:idx_user_key" json:"user_id"`
	Key       string    `gorm:"type:varchar(100);not null;uniqueIndex:idx_user_key" json:"key"`
	Value     JSON      `gorm:"type:json" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Setting) TableName() string { return "settings" }

// --- AnalyticsResult ---

// AnalyticsResult stores the output of an advanced analytics computation.
type AnalyticsResult struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	BacktestRunID int64      `gorm:"index;not null" json:"backtest_run_id"`
	Type          string     `gorm:"type:varchar(30);not null;index" json:"type"`
	Status        string     `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Params        JSON       `gorm:"type:json" json:"params"`
	Result        JSON       `gorm:"type:json" json:"result,omitempty"`
	ErrorMessage  string     `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

func (AnalyticsResult) TableName() string { return "analytics_results" }

const (
	AnalyticsTypeHeatmap     = "heatmap"
	AnalyticsTypeMonteCarlo  = "monte_carlo"
	AnalyticsTypeWalkForward = "walk_forward"

	AnalyticsStatusPending   = "pending"
	AnalyticsStatusRunning   = "running"
	AnalyticsStatusCompleted = "completed"
	AnalyticsStatusFailed    = "failed"
)
