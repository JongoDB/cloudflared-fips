import type { ReactNode } from 'react'

interface LayoutProps {
  children: ReactNode
  version?: string
  cryptoEngine?: string
}

export default function Layout({ children, version, cryptoEngine }: LayoutProps) {
  const versionLabel = version ? `v${version}-fips` : 'v0.0.0-fips'
  const cryptoLabel = cryptoEngine || 'BoringCrypto #4735'

  return (
    <div className="min-h-screen bg-gray-50 overflow-x-hidden">
      <header className="bg-white border-b border-gray-200 shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
            <div className="flex items-center gap-3 min-w-0">
              <img src="/logo.svg" alt="cloudflared-fips" className="w-12 h-8 shrink-0" />
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
              <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                {cryptoLabel}
              </span>
              <span className="text-xs sm:text-sm text-gray-500">{versionLabel}</span>
            </div>
          </div>
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {children}
      </main>
    </div>
  )
}
