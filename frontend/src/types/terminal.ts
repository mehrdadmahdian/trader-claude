// All Bloomberg terminal-specific types
// Shared trading types remain in types/index.ts

export type FunctionCode =
  | 'GP'    // Chart
  | 'HM'    // Heatmap
  | 'FA'    // Fundamentals
  | 'NEWS'  // News
  | 'PORT'  // Portfolio
  | 'WL'    // Watchlist
  | 'SCR'   // Screener
  | 'CAL'   // Calendar
  | 'OPT'   // Options Chain
  | 'YCRV'  // Yield Curves
  | 'RISK'  // Risk Analytics
  | 'BT'    // Backtest
  | 'ALRT'  // Alerts
  | 'MON'   // Monitor
  | 'AI'    // AI Chat

export type LinkGroup = 'red' | 'blue' | 'green' | 'yellow' | null

export interface PanelConfig {
  id: string
  functionCode: FunctionCode
  ticker: string
  market?: string
  timeframe?: string
  params?: Record<string, unknown>
  linkGroup?: LinkGroup
  maximized?: boolean
}

// react-grid-layout grid item shape
export interface GridItem {
  i: string   // panel id
  x: number
  y: number
  w: number
  h: number
  minW?: number
  minH?: number
}

export interface WorkspaceConfig {
  id?: number
  name: string
  layout: GridItem[]
  panels: Record<string, PanelConfig>  // panelId → PanelConfig
}

// Props contract every widget component must satisfy
export interface WidgetProps {
  ticker: string
  market?: string
  timeframe?: string
  params?: Record<string, unknown>
}

// Command bar autocomplete suggestion
export interface CommandSuggestion {
  type: 'ticker' | 'function'
  value: string
  label: string
  description?: string
}

export const FUNCTION_META: Record<FunctionCode, { label: string; description: string }> = {
  GP:   { label: 'Chart',          description: 'Candlestick chart with indicators' },
  HM:   { label: 'Heatmap',        description: 'Market heatmap by asset class' },
  FA:   { label: 'Fundamentals',   description: 'P/E, EPS, revenue, balance sheet' },
  NEWS: { label: 'News',           description: 'Asset-specific news feed' },
  PORT: { label: 'Portfolio',      description: 'Positions, PnL, transactions' },
  WL:   { label: 'Watchlist',      description: 'Multi-column watchlist' },
  SCR:  { label: 'Screener',       description: 'Filter by any metric' },
  CAL:  { label: 'Calendar',       description: 'Earnings & macro events' },
  OPT:  { label: 'Options Chain',  description: 'Put/call chain with Greeks' },
  YCRV: { label: 'Yield Curves',   description: 'US Treasury yield curves' },
  RISK: { label: 'Risk Analytics', description: 'VaR, Sharpe, stress tests' },
  BT:   { label: 'Backtest',       description: 'Strategy backtest runner' },
  ALRT: { label: 'Alerts',         description: 'Price & volume alert rules' },
  MON:  { label: 'Monitor',        description: 'Live strategy monitor' },
  AI:   { label: 'AI Chat',        description: 'AI assistant panel' },
}
