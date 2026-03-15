import { ChatPanel } from '@/components/ai/ChatPanel'
import type { WidgetProps } from '@/types/terminal'

export function AIChatWidget(_: WidgetProps) {
  return (
    <div className="h-full flex flex-col overflow-hidden">
      <ChatPanel isOpen={true} onClose={() => {}} />
    </div>
  )
}
