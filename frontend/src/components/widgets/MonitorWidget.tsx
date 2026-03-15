import { Monitor } from '@/pages/Monitor'
import type { WidgetProps } from '@/types/terminal'

export function MonitorWidget(_: WidgetProps) {
  return (
    <div className="h-full overflow-y-auto">
      <Monitor />
    </div>
  )
}
