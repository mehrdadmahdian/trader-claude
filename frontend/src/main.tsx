import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { App } from './App'
import './index.css'

// Apply persisted theme before first render to prevent flash
const savedTheme = localStorage.getItem('trader-theme')
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
let theme = 'dark'
try {
  const parsed = savedTheme ? JSON.parse(savedTheme) : null
  theme = parsed?.state?.theme ?? (prefersDark ? 'dark' : 'light')
} catch {
  theme = prefersDark ? 'dark' : 'light'
}
document.documentElement.classList.toggle('dark', theme === 'dark')

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
)
