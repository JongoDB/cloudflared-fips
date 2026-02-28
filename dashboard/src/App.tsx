import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
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

  // Show nothing while detecting mode
  if (fleetMode === null) {
    return null
  }

  return (
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
  )
}

export default App
