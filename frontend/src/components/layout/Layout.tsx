import { useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { CommandBar } from './CommandBar'
import { useNotificationWS } from '@/hooks/useNotifications'
import { SignalToast } from '@/components/SignalToast'
import { AIButton } from '@/components/ai/AIButton'
import { ChatPanel } from '@/components/ai/ChatPanel'
import { cn } from '@/lib/utils'

export function Layout() {
  const [isChatOpen, setIsChatOpen] = useState(false)
  const { pathname } = useLocation()
  useNotificationWS()

  const isDashboard = pathname === '/'

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <CommandBar />
      <main className={cn('flex-1 overflow-hidden', !isDashboard && 'overflow-y-auto')}>
        {isDashboard ? (
          <Outlet />
        ) : (
          <div className="max-w-7xl mx-auto px-6 py-6 animate-fade-in">
            <Outlet />
          </div>
        )}
      </main>
      <SignalToast />
      <AIButton onClick={() => setIsChatOpen((o) => !o)} isOpen={isChatOpen} />
      <ChatPanel isOpen={isChatOpen} onClose={() => setIsChatOpen(false)} />
    </div>
  )
}
