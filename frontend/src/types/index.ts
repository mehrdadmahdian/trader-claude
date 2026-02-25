// ── Core market types ──────────────────────────────────────────────────────

export interface Candle {
  id: number
  symbol: string
  market: string
  timeframe: string
  timestamp: string
  open: number
  high: number
  low: number
  close: number
  volume: number
}

export interface Tick {
  symbol: string
  market: string
  price: number
  volume: number
  timestamp: string
  bid: number
  ask: number
}

export interface Symbol {
  id: number
  ticker: string
  market: string
  base_asset: string
  quote_asset: string
  description: string
  active: boolean
}

// ── Market API types (Phase 2) ─────────────────────────────────────────────

export interface MarketAdapter {
  id: string
  markets: string[]
  healthy: boolean
}

export interface MarketSymbol {
  id: string
  market: string
  base_asset: string
  quote_asset: string
  description: string
  active: boolean
}

// OHLCVCandle from the API (timestamp is Unix seconds)
export interface OHLCVCandle {
  symbol: string
  market: string
  timeframe: string
  timestamp: number
  open: number
  high: number
  low: number
  close: number
  volume: number
}

// ── Strategy types ─────────────────────────────────────────────────────────

export interface ParamDefinition {
  name: string
  type: 'int' | 'float' | 'bool' | 'string' | 'select'
  default: unknown
  min?: number
  max?: number
  options?: string[]
  description: string
  required: boolean
}

export interface StrategyDef {
  id: number
  name: string
  description: string
  params_schema: ParamDefinition[]
  active: boolean
}

// StrategyInfo is returned by GET /api/v1/strategies
export interface StrategyInfo {
  id: string
  name: string
  description: string
  params: ParamDefinition[]
}

// ── Backtest types ─────────────────────────────────────────────────────────

export type BacktestStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'

export interface BacktestMetrics {
  total_return: number
  annualized_return: number
  sharpe_ratio: number
  sortino_ratio: number
  max_drawdown: number
  max_drawdown_duration_seconds: number
  win_rate: number
  profit_factor: number
  avg_win: number
  avg_loss: number
  total_trades: number
  winning_trades: number
  losing_trades: number
  largest_win: number
  largest_loss: number
}

export interface EquityPoint {
  timestamp: string
  value: number
}

export interface Backtest {
  id: number
  name: string
  strategy_name: string
  symbol: string
  market: string
  timeframe: string
  start_date: string
  end_date: string
  params: Record<string, unknown>
  status: BacktestStatus
  metrics?: BacktestMetrics
  equity_curve?: EquityPoint[]
  error_message?: string
  started_at?: string
  completed_at?: string
  created_at: string
}

export interface BacktestCreateRequest {
  name: string
  strategy_name: string
  symbol: string
  market: string
  timeframe: string
  start_date: string
  end_date: string
  params: Record<string, unknown>
}

export interface BacktestRunRequest {
  name: string
  strategy: string
  adapter: string
  symbol: string
  market: string
  timeframe: string
  start_date: string
  end_date: string
  params: Record<string, unknown>
  initial_cash?: number
  commission?: number
}

// ── Trade types ────────────────────────────────────────────────────────────

export type TradeDirection = 'long' | 'short'

export interface Trade {
  id: number
  backtest_id?: number
  portfolio_id?: number
  symbol: string
  market: string
  direction: TradeDirection
  entry_price: number
  exit_price?: number
  quantity: number
  entry_time: string
  exit_time?: string
  pnl?: number
  pnl_percent?: number
  fee: number
}

// ── Portfolio types ────────────────────────────────────────────────────────

export interface Portfolio {
  id: number
  name: string
  strategy_name: string
  symbol: string
  market: string
  timeframe: string
  params: Record<string, unknown>
  is_live: boolean
  is_active: boolean
  initial_cash: number
  current_cash: number
  current_value: number
}

// ── Alert types ────────────────────────────────────────────────────────────

export type AlertStatus = 'active' | 'triggered' | 'disabled'
export type AlertCondition = 'price_above' | 'price_below' | 'price_change_pct' | 'volume_spike' | 'custom'

export interface Alert {
  id: number
  name: string
  symbol: string
  market: string
  condition: AlertCondition
  threshold: number
  status: AlertStatus
  message: string
  triggered_at?: string
  created_at: string
}

export interface AlertCreateRequest {
  name: string
  symbol: string
  market: string
  condition: AlertCondition
  threshold: number
  message?: string
}

// ── Notification types ─────────────────────────────────────────────────────

export type NotificationType = 'alert' | 'trade' | 'system' | 'backtest'

export interface Notification {
  id: number
  type: NotificationType
  title: string
  body: string
  read: boolean
  created_at: string
}

// ── WatchList types ────────────────────────────────────────────────────────

export interface WatchList {
  id: number
  name: string
  symbols: string[]
  created_at: string
  updated_at: string
}

// ── WebSocket types ────────────────────────────────────────────────────────

export type WsMessageType = 'tick' | 'candle' | 'signal' | 'alert' | 'notification' | 'error' | 'ping' | 'pong'

export interface WsMessage<T = unknown> {
  type: WsMessageType
  channel?: string
  payload: T
}

// ── API response wrappers ──────────────────────────────────────────────────

export interface ApiResponse<T> {
  data: T
  message?: string
}

export interface ApiError {
  error: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  page_size: number
}

// ── Health ─────────────────────────────────────────────────────────────────

export interface HealthResponse {
  status: string
  db: string
  redis: string
  version: string
}

// ── Replay types ────────────────────────────────────────────────────────────

export type ReplayStatus = 'idle' | 'playing' | 'paused' | 'complete'

export interface ReplayCandle {
  symbol: string
  market: string
  timeframe: string
  timestamp: string
  open: number
  high: number
  low: number
  close: number
  volume: number
}

export interface ReplayEquityPoint {
  timestamp: string
  value: number
}

export interface ReplayTradeEvent {
  trade: Trade
}

export interface ReplaySeekSnapshot {
  candles: ReplayCandle[]
  equity: ReplayEquityPoint[]
  trades: Trade[]
}

export interface ReplayState {
  state: ReplayStatus
  index: number
  total: number
  speed: number
}

// Messages sent FROM server TO client
export interface ReplayServerMsg {
  type: 'candle' | 'trade_open' | 'trade_close' | 'equity_update' | 'seek_snapshot' | 'status' | 'error'
  data: ReplayCandle | ReplayTradeEvent | ReplayEquityPoint | ReplaySeekSnapshot | ReplayState | string
}

// Messages sent FROM client TO server
export interface ReplayControlMsg {
  type: 'start' | 'resume' | 'pause' | 'step' | 'set_speed' | 'seek'
  speed?: number
  index?: number
}

export interface ReplayBookmark {
  id: number
  user_id: number
  backtest_run_id: number
  candle_index: number
  label: string
  note: string
  chart_snapshot: string
  created_at: string
}

export interface CreateBookmarkRequest {
  backtest_run_id: number
  candle_index: number
  label: string
  note: string
  chart_snapshot: string
}

// ── Indicator types (Phase 5) ───────────────────────────────────────────────

export type IndicatorType = 'overlay' | 'panel'
export type IndicatorGroup = 'trend' | 'momentum' | 'volatility' | 'volume'

export interface OutputDef {
  name: string
  color: string
}

export interface IndicatorMeta {
  id: string
  name: string
  full_name: string
  type: IndicatorType
  group: IndicatorGroup
  params: ParamDefinition[]  // reuses the existing ParamDefinition interface
  outputs: OutputDef[]
}

export interface CalcResult {
  timestamps: number[]
  series: Record<string, (number | null)[]>
}

export interface ActiveIndicator {
  meta: IndicatorMeta
  params: Record<string, unknown>
  result?: CalcResult
}

export interface CalculateRequest {
  indicator_id: string
  params: Record<string, unknown>
  candles: Array<{
    timestamp: number
    open: number
    high: number
    low: number
    close: number
    volume: number
  }>
}
