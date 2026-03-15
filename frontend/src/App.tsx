import { Routes, Route } from 'react-router-dom'
import { useEffect } from 'react'
import { Layout } from '@/components/layout/Layout'
import { WorkspaceLayout } from '@/components/terminal/WorkspaceLayout'
import ProtectedRoute from '@/components/auth/ProtectedRoute'
import { Dashboard } from '@/pages/Dashboard'
import { Chart } from '@/pages/Chart'
import { Backtest } from '@/pages/Backtest'
import { Portfolio } from '@/pages/Portfolio'
import { Monitor } from '@/pages/Monitor'
import { News } from '@/pages/News'
import { Alerts } from '@/pages/Alerts'
import { Settings } from '@/pages/Settings'
import { Notifications } from '@/pages/Notifications'
import Login from '@/pages/Login'
import Register from '@/pages/Register'
import { useAuthStore } from '@/stores/authStore'

export function App() {
  const initialize = useAuthStore((s) => s.initialize)

  useEffect(() => {
    initialize()
  }, [initialize])

  return (
    <Routes>
      {/* Public routes */}
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />

      {/* Protected routes — require authentication */}
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route path="/" element={<Dashboard />} />
        <Route path="/chart" element={<Chart />} />
        <Route path="/backtest" element={<Backtest />} />
        <Route path="/portfolio" element={<Portfolio />} />
        <Route path="/monitor" element={<Monitor />} />
        <Route path="/news" element={<News />} />
        <Route path="/alerts" element={<Alerts />} />
        <Route path="/notifications" element={<Notifications />} />
        <Route path="/settings" element={<Settings />} />
      </Route>

      {/* Bloomberg terminal — standalone layout (no sidebar) */}
      <Route
        path="/terminal"
        element={
          <ProtectedRoute>
            <WorkspaceLayout />
          </ProtectedRoute>
        }
      />
    </Routes>
  )
}
