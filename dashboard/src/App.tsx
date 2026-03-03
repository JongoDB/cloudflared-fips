import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
import ErrorBoundary from './components/ErrorBoundary'
import DashboardPage from './pages/DashboardPage'
import FleetOverviewPage from './pages/FleetOverviewPage'
import NodeDetailPage from './pages/NodeDetailPage'

function App() {
  const [fleetMode, setFleetMode] = useState<boolean | null>(null)

  // Detect fleet mode by probing the fleet API
  useEffect(() => {
    fetch('/api/v1/fleet/summary')
      .then((res) => {
        setFleetMode(res.ok)
      })
      .catch(() => {
        setFleetMode(false)
      })
  }, [])

  // Loading indicator while detecting mode
  if (fleetMode === null) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <div className="w-8 h-8 border-2 border-blue-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
          <p className="text-sm text-gray-500">Loading dashboard...</p>
        </div>
      </div>
    )
  }

  return (
    <ErrorBoundary>
      <BrowserRouter>
        <Routes>
          {/* Fleet mode routes */}
          <Route path="/fleet" element={<FleetOverviewPage />} />
          <Route path="/fleet/nodes/:id" element={<NodeDetailPage />} />

          {/* Local node compliance */}
          <Route path="/node" element={<DashboardPage />} />

          {/* Root: redirect based on fleet mode */}
          <Route
            path="/"
            element={<Navigate to={fleetMode ? '/fleet' : '/node'} replace />}
          />

          {/* Fallback */}
          <Route
            path="*"
            element={<Navigate to={fleetMode ? '/fleet' : '/node'} replace />}
          />
        </Routes>
      </BrowserRouter>
    </ErrorBoundary>
  )
}

export default App
