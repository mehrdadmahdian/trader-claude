import { Routes, Route } from 'react-router-dom'
import { Layout } from '@/components/layout/Layout'
import { Dashboard } from '@/pages/Dashboard'
import { Chart } from '@/pages/Chart'
import { Backtest } from '@/pages/Backtest'
import { Portfolio } from '@/pages/Portfolio'
import { Monitor } from '@/pages/Monitor'
import { News } from '@/pages/News'
import { Alerts } from '@/pages/Alerts'
import { Settings } from '@/pages/Settings'
import { Notifications } from '@/pages/Notifications'

export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
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
    </Routes>
  )
}
