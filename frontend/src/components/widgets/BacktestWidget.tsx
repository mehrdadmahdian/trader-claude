import { Backtest } from '@/pages/Backtest'
import type { WidgetProps } from '@/types/terminal'

export function BacktestWidget(_: WidgetProps) {
  return (
    <div className="h-full overflow-y-auto">
      <Backtest />
    </div>
  )
}
