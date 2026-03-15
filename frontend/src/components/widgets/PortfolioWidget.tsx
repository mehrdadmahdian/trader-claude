import { Portfolio } from '@/pages/Portfolio'
import type { WidgetProps } from '@/types/terminal'

export function PortfolioWidget(_: WidgetProps) {
  return (
    <div className="h-full overflow-y-auto">
      <Portfolio />
    </div>
  )
}
