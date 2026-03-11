import React from 'react'
import type { FunctionCode, WidgetProps } from '@/types/terminal'

// Phase A — real widgets (adapted wrappers)
import { ChartWidget }     from '@/components/widgets/ChartWidget'
import { NewsWidget }      from '@/components/widgets/NewsWidget'
import { PortfolioWidget } from '@/components/widgets/PortfolioWidget'
import { WatchlistWidget } from '@/components/widgets/WatchlistWidget'
import { AlertsWidget }    from '@/components/widgets/AlertsWidget'
import { BacktestWidget }  from '@/components/widgets/BacktestWidget'
import { MonitorWidget }   from '@/components/widgets/MonitorWidget'
import { AIChatWidget }    from '@/components/widgets/AIChatWidget'

// Stub for future phases
const ComingSoon = ({ label }: { label: string }) => (
  <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
    {label} — coming soon
  </div>
)

export const WIDGET_REGISTRY: Record<FunctionCode, React.ComponentType<WidgetProps>> = {
  // Phase A: research & market widgets
  GP:   ChartWidget,
  NEWS: NewsWidget,
  PORT: PortfolioWidget,
  WL:   WatchlistWidget,

  // Read-only views of workbench tools
  ALRT: AlertsWidget,
  BT:   BacktestWidget,
  MON:  MonitorWidget,
  AI:   AIChatWidget,

  // Phases B-H: stubbed for now
  HM:   () => <ComingSoon label="Market Heatmap (Phase B)" />,
  FA:   () => <ComingSoon label="Fundamentals (Phase D)" />,
  SCR:  () => <ComingSoon label="Screener (Phase C)" />,
  CAL:  () => <ComingSoon label="Calendar (Phase E)" />,
  OPT:  () => <ComingSoon label="Options Chain (Phase G)" />,
  YCRV: () => <ComingSoon label="Yield Curves (Phase F)" />,
  RISK: () => <ComingSoon label="Risk Analytics (Phase H)" />,
}

export function getWidget(code: FunctionCode): React.ComponentType<WidgetProps> {
  return WIDGET_REGISTRY[code] ?? (() => <ComingSoon label={code} />)
}
