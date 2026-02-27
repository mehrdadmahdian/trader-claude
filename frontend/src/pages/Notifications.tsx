import { useState } from 'react'
import { Bell, CheckCheck, Circle } from 'lucide-react'
import { formatDistanceToNow, format } from 'date-fns'
import { useNotifications, useMarkRead, useMarkAllRead } from '@/hooks/useNotifications'
import { cn } from '@/lib/utils'
import type { NotificationType } from '@/types'

const PAGE_SIZE = 20

function TypeBadge({ type }: { type: NotificationType }) {
  const colors: Record<NotificationType, string> = {
    alert: 'bg-yellow-500/15 text-yellow-600 dark:text-yellow-400',
    trade: 'bg-blue-500/15 text-blue-600 dark:text-blue-400',
    system: 'bg-muted text-muted-foreground',
    backtest: 'bg-purple-500/15 text-purple-600 dark:text-purple-400',
  }
  return (
    <span
      className={cn(
        'inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide',
        colors[type],
      )}
    >
      {type}
    </span>
  )
}

export function Notifications() {
  const [page, setPage] = useState(1)
  const { data, isLoading, isError } = useNotifications(page, PAGE_SIZE)
  const { mutate: markRead } = useMarkRead()
  const { mutate: markAllRead } = useMarkAllRead()

  const notifications = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div>
      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Notifications</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {total} total · {data?.data.filter((n) => !n.read).length ?? 0} unread
          </p>
        </div>
        <button
          onClick={() => markAllRead()}
          className="flex items-center gap-2 px-4 py-2 text-sm border border-border rounded-md hover:bg-accent transition-colors"
        >
          <CheckCheck className="w-4 h-4" />
          Mark all read
        </button>
      </div>

      {/* Loading / error / empty */}
      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading notifications…</p>
      )}
      {isError && (
        <p className="text-sm text-destructive">Failed to load notifications.</p>
      )}
      {!isLoading && notifications.length === 0 && (
        <div className="text-center py-16 text-muted-foreground">
          <Bell className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p className="font-medium">No notifications yet</p>
        </div>
      )}

      {/* Notification list */}
      {notifications.length > 0 && (
        <div className="bg-card border border-border rounded-lg overflow-hidden divide-y divide-border">
          {notifications.map((n) => (
            <div
              key={n.id}
              className={cn(
                'flex gap-3 px-4 py-4 transition-colors',
                !n.read && 'bg-primary/5',
                'hover:bg-accent/30',
              )}
            >
              {/* Unread indicator */}
              <div className="flex-shrink-0 mt-1">
                {n.read ? (
                  <Circle className="w-2 h-2 text-muted-foreground/30" />
                ) : (
                  <Circle className="w-2 h-2 text-primary fill-primary" />
                )}
              </div>

              {/* Content */}
              <div className="flex-1 min-w-0">
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-center gap-2 flex-wrap">
                    <p className="text-sm font-medium">{n.title}</p>
                    <TypeBadge type={n.type} />
                  </div>
                  <time
                    className="text-xs text-muted-foreground shrink-0"
                    title={format(new Date(n.created_at), 'PPpp')}
                  >
                    {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                  </time>
                </div>
                <p className="text-sm text-muted-foreground mt-0.5">{n.body}</p>
              </div>

              {/* Mark read action */}
              {!n.read && (
                <button
                  onClick={() => markRead(n.id)}
                  className="shrink-0 px-2 py-1 text-xs text-muted-foreground hover:text-foreground border border-border rounded hover:bg-accent transition-colors"
                  aria-label="Mark as read"
                >
                  Read
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 mt-6">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className="px-3 py-1.5 text-sm border border-border rounded-md hover:bg-accent transition-colors disabled:opacity-40"
          >
            Previous
          </button>
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page === totalPages}
            className="px-3 py-1.5 text-sm border border-border rounded-md hover:bg-accent transition-colors disabled:opacity-40"
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}
