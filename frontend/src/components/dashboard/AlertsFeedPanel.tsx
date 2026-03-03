import { Bell } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { useAlerts } from '@/hooks/useAlerts'
import { useAlertStore, useNotificationStore } from '@/stores'
import type { Alert, Notification } from '@/types'

// ── Helpers ─────────────────────────────────────────────────────────────────

function timeAgo(dateStr: string): string {
  try {
    return formatDistanceToNow(new Date(dateStr), { addSuffix: true })
  } catch {
    return ''
  }
}

function conditionLabel(condition: Alert['condition']): string {
  switch (condition) {
    case 'price_above':
      return 'Price above'
    case 'price_below':
      return 'Price below'
    case 'price_change_pct':
      return 'Price change %'
    case 'volume_spike':
      return 'Volume spike'
    case 'custom':
      return 'Custom'
  }
}

// ── Icon circle ─────────────────────────────────────────────────────────────

function IconCircle({ color }: { color: 'amber' | 'blue' | 'slate' }) {
  const colorMap: Record<typeof color, string> = {
    amber: 'bg-amber-100 text-amber-500',
    blue: 'bg-blue-100 text-blue-500',
    slate: 'bg-slate-100 text-slate-400',
  }
  return (
    <span
      className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-full ${colorMap[color]}`}
    >
      <Bell className="h-3.5 w-3.5" />
    </span>
  )
}

// ── Alert row ───────────────────────────────────────────────────────────────

function AlertRow({ alert }: { alert: Alert }) {
  return (
    <li className="flex items-start gap-2.5 px-3 py-2.5 hover:bg-slate-50 transition-colors cursor-default">
      <IconCircle color="amber" />
      <div className="flex-1 min-w-0">
        <p className="text-xs font-semibold text-slate-700 truncate">
          {alert.name}
        </p>
        <p className="text-[10px] text-slate-400 mt-0.5">
          {alert.symbol} &middot; {conditionLabel(alert.condition)} {alert.threshold}
        </p>
      </div>
      <span className="text-[10px] text-slate-400 shrink-0 mt-0.5">
        {timeAgo(alert.created_at)}
      </span>
    </li>
  )
}

// ── Notification row ────────────────────────────────────────────────────────

function NotificationRow({ notification }: { notification: Notification }) {
  const iconColor: 'blue' | 'amber' | 'slate' =
    notification.type === 'alert' ? 'amber' : 'blue'

  return (
    <li className="flex items-start gap-2.5 px-3 py-2.5 hover:bg-slate-50 transition-colors cursor-default">
      <IconCircle color={iconColor} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <p className="text-xs font-semibold text-slate-700 truncate">
            {notification.title}
          </p>
          {!notification.read && (
            <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-blue-500" />
          )}
        </div>
        <p className="text-[10px] text-slate-400 mt-0.5 line-clamp-2">
          {notification.body}
        </p>
      </div>
      <span className="text-[10px] text-slate-400 shrink-0 mt-0.5">
        {timeAgo(notification.created_at)}
      </span>
    </li>
  )
}

// ── Empty state ─────────────────────────────────────────────────────────────

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-10 text-slate-400">
      <Bell className="h-7 w-7 opacity-40" />
      <p className="text-xs">No alerts yet</p>
    </div>
  )
}

// ── Main panel ──────────────────────────────────────────────────────────────

export default function AlertsFeedPanel() {
  // Populate the alert store via React Query
  useAlerts()

  const alerts = useAlertStore((s) => s.alerts)
  const notifications = useNotificationStore((s) => s.notifications)

  const activeAlerts = alerts.filter((a) => a.status === 'active')
  const recentNotifications = notifications.slice(0, 3)
  const topAlerts = activeAlerts.slice(0, 5)

  const hasItems = recentNotifications.length > 0 || topAlerts.length > 0

  return (
    <div className="flex flex-col h-full bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-slate-100 shrink-0">
        <div className="flex items-center gap-1.5">
          <Bell className="h-3.5 w-3.5 text-slate-400" />
          <span className="text-[11px] font-semibold text-slate-500 tracking-wider uppercase">
            Alerts
          </span>
          {activeAlerts.length > 0 && (
            <span className="inline-flex items-center rounded-full bg-amber-50 px-2 py-0.5 text-[10px] font-semibold text-amber-600 border border-amber-100">
              {activeAlerts.length} active
            </span>
          )}
        </div>
      </div>

      {/* Scrollable list */}
      <div className="flex-1 overflow-y-auto min-h-0">
        {!hasItems ? (
          <EmptyState />
        ) : (
          <ul className="divide-y divide-slate-100">
            {recentNotifications.map((n) => (
              <NotificationRow key={`notif-${n.id}`} notification={n} />
            ))}
            {topAlerts.map((a) => (
              <AlertRow key={`alert-${a.id}`} alert={a} />
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
