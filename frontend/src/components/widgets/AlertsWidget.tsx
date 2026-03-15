import { Alerts } from '@/pages/Alerts'
import type { WidgetProps } from '@/types/terminal'

export function AlertsWidget(_: WidgetProps) {
  return (
    <div className="h-full overflow-y-auto p-3">
      <Alerts />
    </div>
  )
}
