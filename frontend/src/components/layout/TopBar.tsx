import { useNavigate } from 'react-router-dom'
import { Menu, Moon, Sun, Bell, CheckCheck, LogOut } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { useThemeStore, useNotificationStore, useSidebarStore } from '@/stores'
import { useAuthStore } from '@/stores/authStore'
import { useMarkAllRead } from '@/hooks/useNotifications'
import { cn } from '@/lib/utils'

export function TopBar() {
  const { theme, toggleTheme } = useThemeStore()
  const { unreadCount, notifications } = useNotificationStore()
  const { toggle } = useSidebarStore()
  const navigate = useNavigate()
  const { mutate: markAllRead } = useMarkAllRead()
  const { user, logout } = useAuthStore()

  // Show last 5 notifications in dropdown (most recent first)
  const recentNotifications = notifications.slice(0, 5)

  return (
    <header className="h-16 border-b border-border bg-card flex items-center gap-4 px-4 shrink-0">
      {/* Mobile hamburger */}
      <button
        onClick={toggle}
        className="lg:hidden p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        aria-label="Toggle sidebar"
      >
        <Menu className="w-5 h-5" />
      </button>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Theme toggle */}
      <button
        onClick={toggleTheme}
        className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
      >
        {theme === 'dark' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
      </button>

      {/* Notification bell with dropdown */}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button
            className="relative p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
            aria-label="Notifications"
          >
            <Bell className="w-5 h-5" />
            {unreadCount > 0 && (
              <span
                className={cn(
                  'absolute top-1 right-1 min-w-[1rem] h-4 px-0.5',
                  'flex items-center justify-center',
                  'text-[10px] font-bold text-white bg-destructive rounded-full',
                )}
              >
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </button>
        </DropdownMenu.Trigger>

        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={8}
            className={cn(
              'z-50 w-80 rounded-lg border border-border bg-card shadow-lg',
              'animate-in fade-in-0 zoom-in-95',
            )}
          >
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 border-b border-border">
              <span className="font-semibold text-sm">Notifications</span>
              {unreadCount > 0 && (
                <button
                  onClick={() => markAllRead()}
                  className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                  aria-label="Mark all as read"
                >
                  <CheckCheck className="w-3.5 h-3.5" />
                  Mark all read
                </button>
              )}
            </div>

            {/* Notification list */}
            {recentNotifications.length === 0 ? (
              <div className="px-4 py-6 text-center text-sm text-muted-foreground">
                No notifications yet
              </div>
            ) : (
              <div>
                {recentNotifications.map((n) => (
                  <DropdownMenu.Item
                    key={n.id}
                    className={cn(
                      'flex flex-col gap-0.5 px-4 py-3 border-b border-border last:border-0',
                      'cursor-default select-none outline-none',
                      'hover:bg-accent transition-colors',
                      !n.read && 'bg-primary/5',
                    )}
                  >
                    <div className="flex items-start gap-2">
                      {!n.read && (
                        <span className="mt-1.5 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
                      )}
                      <div className={cn('min-w-0', n.read && 'pl-3.5')}>
                        <p className="text-xs font-medium truncate">{n.title}</p>
                        <p className="text-xs text-muted-foreground line-clamp-2 mt-0.5">
                          {n.body}
                        </p>
                        <p className="text-[10px] text-muted-foreground mt-1">
                          {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                        </p>
                      </div>
                    </div>
                  </DropdownMenu.Item>
                ))}
              </div>
            )}

            {/* Footer */}
            <DropdownMenu.Item
              className="flex justify-center px-4 py-2.5 text-xs text-primary hover:text-primary/80 hover:bg-accent transition-colors cursor-pointer outline-none"
              onSelect={() => navigate('/notifications')}
            >
              View all notifications →
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>

      {/* User avatar dropdown */}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button
            className="flex items-center justify-center w-8 h-8 rounded-full bg-primary/20 text-primary text-sm font-semibold hover:bg-primary/30 transition-colors"
            aria-label="User menu"
          >
            {user ? (user.display_name || user.email).charAt(0).toUpperCase() : '?'}
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={8}
            className="z-50 w-48 rounded-lg border border-border bg-card shadow-lg animate-in fade-in-0 zoom-in-95"
          >
            {user && (
              <div className="px-3 py-2 border-b border-border">
                <p className="text-xs font-medium truncate">{user.display_name || user.email}</p>
                <p className="text-[10px] text-muted-foreground capitalize">{user.role}</p>
              </div>
            )}
            <DropdownMenu.Item
              className="flex items-center gap-2 px-3 py-2 text-sm text-destructive hover:bg-destructive/10 cursor-pointer outline-none transition-colors"
              onSelect={() => logout()}
            >
              <LogOut className="w-4 h-4" />
              Logout
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </header>
  )
}
