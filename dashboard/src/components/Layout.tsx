import { useState, useEffect } from 'react'
import type { ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import type { BuildManifest } from '../types/compliance'

interface LayoutProps {
  children: ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const [version, setVersion] = useState('')
  const [cryptoLabel, setCryptoLabel] = useState('')
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    fetch('/api/v1/manifest')
      .then((res) => (res.ok ? res.json() : null))
      .then((data: BuildManifest | null) => {
        if (data?.version) {
          const v = data.version.startsWith('v') ? data.version : `v${data.version}`
          setVersion(`${v}-fips`)
        }
        if (data?.crypto_engine) {
          const cert = data.fips_certificates?.[0]?.certificate
          setCryptoLabel(cert ? `${data.crypto_engine} ${cert}` : data.crypto_engine)
        }
      })
      .catch(() => {})
  }, [])

  const isFleetPage = location.pathname.startsWith('/fleet')
  const isNodePage = location.pathname === '/node' || location.pathname === '/'

  return (
    <div className="min-h-screen bg-gray-50 overflow-x-hidden">
      <header className="bg-white border-b border-gray-200 shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
            <div className="flex items-center gap-3 min-w-0">
              <img src="/logo.svg" alt="cloudflared-fips" className="w-16 h-11 shrink-0" />
              <div className="min-w-0">
                <h1 className="text-lg sm:text-xl font-semibold text-gray-900 truncate">
                  cloudflared-fips
                </h1>
                <p className="text-xs sm:text-sm text-gray-500">
                  FIPS 140-3 Compliance Dashboard
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 sm:gap-4">
              {cryptoLabel && (
                <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                  {cryptoLabel}
                </span>
              )}
              {version && (
                <span className="text-xs sm:text-sm text-gray-500">{version}</span>
              )}
            </div>
          </div>
        </div>
        {/* Navigation tabs */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <nav className="flex gap-1 -mb-px">
            <button
              onClick={() => navigate('/fleet')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                isFleetPage
                  ? 'border-blue-600 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              Fleet
            </button>
            <button
              onClick={() => navigate('/node')}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                isNodePage
                  ? 'border-blue-600 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              Compliance
            </button>
          </nav>
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {children}
      </main>
    </div>
  )
}
