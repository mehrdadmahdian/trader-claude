import { useNotificationWS } from '@/hooks/useNotifications'
import { SignalToast } from '@/components/SignalToast'
import { CommandBar } from './CommandBar'
import { WorkspaceTabs } from './WorkspaceTabs'
import { PanelGrid } from './PanelGrid'

export function WorkspaceLayout() {
  useNotificationWS()

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <CommandBar />
      <WorkspaceTabs />
      <main className="flex-1 overflow-hidden relative">
        <PanelGrid />
      </main>
      <SignalToast />
    </div>
  )
}
