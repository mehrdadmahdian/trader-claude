import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Derives the WebSocket base URL from the current page host so that
// connections go through the Vite dev proxy (or whatever reverse proxy
// serves the app) without needing a hardcoded VITE_WS_URL env var.
export function wsBase(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}`
}
