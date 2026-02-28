import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'
import { useNotificationWS } from '@/hooks/useNotifications'
import { SignalToast } from '@/components/SignalToast'
import { AIButton } from '@/components/ai/AIButton'
import { ChatPanel } from '@/components/ai/ChatPanel'

export function Layout() {
  const [isChatOpen, setIsChatOpen] = useState(false)
  useNotificationWS() // connect to /ws/notifications on mount

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <Sidebar />
      <div className="flex flex-col flex-1 overflow-hidden">
        <TopBar />
        <main className="flex-1 overflow-y-auto p-6 animate-fade-in">
          <Outlet />
        </main>
      </div>
      <SignalToast />
      <AIButton onClick={() => setIsChatOpen(o => !o)} isOpen={isChatOpen} />
      <ChatPanel isOpen={isChatOpen} onClose={() => setIsChatOpen(false)} />
    </div>
  )
}
