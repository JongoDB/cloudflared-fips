import { useState, useEffect } from 'react'

interface FIPSBackendInfo {
  name: string
  display_name: string
  cmvp_certificate: string
  fips_standard: string
  validated: boolean
  active: boolean
}

interface FIPSBackendCardProps {
  /** If provided, uses this static data instead of fetching from API */
  backendInfo?: FIPSBackendInfo
}

const defaultBackend: FIPSBackendInfo = {
  name: 'boringcrypto',
  display_name: 'BoringCrypto (BoringSSL)',
  cmvp_certificate: '#4407 (140-2) / #4735 (140-3)',
  fips_standard: '140-2',
  validated: true,
  active: true,
}

export default function FIPSBackendCard({ backendInfo }: FIPSBackendCardProps) {
  const [info, setInfo] = useState<FIPSBackendInfo>(backendInfo ?? defaultBackend)
  const [expanded, setExpanded] = useState(false)

  useEffect(() => {
    if (backendInfo) {
      setInfo(backendInfo)
      return
    }
    // Try fetching from API; fall back to default mock data
    fetch('/api/v1/backend')
      .then((res) => {
        if (res.ok) return res.json()
        throw new Error('API unavailable')
      })
      .then((data: FIPSBackendInfo) => setInfo(data))
      .catch(() => {
        // Keep default mock data
      })
  }, [backendInfo])

  const standardBadge = getStandardBadge(info.fips_standard)
  const validationBadge = info.validated
    ? { text: 'Validated', color: 'bg-green-100 text-green-800 border-green-200' }
    : { text: 'Pending', color: 'bg-yellow-100 text-yellow-800 border-yellow-200' }

  return (
    <div className="mb-6 bg-white rounded-lg border border-gray-200 shadow-sm">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-6 py-4 flex items-center justify-between text-left hover:bg-gray-50 transition-colors"
      >
        <div className="flex items-center gap-3">
          <div className="flex-shrink-0">
            <svg className="w-8 h-8 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
                d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
              />
            </svg>
          </div>
          <div>
            <h3 className="text-base font-semibold text-gray-900">
              FIPS Cryptographic Module
            </h3>
            <p className="text-sm text-gray-500">
              {info.active ? info.display_name : 'No FIPS module detected'}
            </p>
          </div>
          <div className="flex items-center gap-2 ml-4">
            <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${standardBadge.color}`}>
              {standardBadge.text}
            </span>
            <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${validationBadge.color}`}>
              {validationBadge.text}
            </span>
          </div>
        </div>
        <svg
          className={`w-5 h-5 text-gray-400 transition-transform ${expanded ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {expanded && (
        <div className="px-6 pb-6 border-t border-gray-100">
          <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Module Details
              </h4>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">Backend</dt>
                  <dd className="text-gray-900 font-mono">{info.name}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Display Name</dt>
                  <dd className="text-gray-900">{info.display_name}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">FIPS Standard</dt>
                  <dd className="text-gray-900 font-mono">{info.fips_standard}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">CMVP Certificate</dt>
                  <dd className="text-gray-900 font-mono">{info.cmvp_certificate}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Active</dt>
                  <dd className={info.active ? 'text-green-700 font-medium' : 'text-red-700 font-medium'}>
                    {info.active ? 'Yes' : 'No'}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">CMVP Validated</dt>
                  <dd className={info.validated ? 'text-green-700 font-medium' : 'text-yellow-700 font-medium'}>
                    {info.validated ? 'Yes' : 'Pending'}
                  </dd>
                </div>
              </dl>
            </div>
            <div>
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Validation Info
              </h4>
              {info.name === 'boringcrypto' && (
                <div className="space-y-2 text-sm text-gray-600">
                  <p>BoringCrypto is Google's FIPS-validated BoringSSL module, statically linked via <code className="bg-gray-100 px-1 rounded">GOEXPERIMENT=boringcrypto</code>.</p>
                  <p><strong>FIPS 140-2:</strong> CMVP #3678, #4407</p>
                  <p><strong>FIPS 140-3:</strong> CMVP #4735</p>
                  <p><strong>Platform:</strong> Linux amd64/arm64 only</p>
                </div>
              )}
              {info.name === 'go-native' && (
                <div className="space-y-2 text-sm text-gray-600">
                  <p>Go Cryptographic Module (Go 1.24+) provides native FIPS 140-3 support via <code className="bg-gray-100 px-1 rounded">GODEBUG=fips140=on</code>.</p>
                  <p><strong>CAVP:</strong> A6650</p>
                  <p><strong>CMVP:</strong> Pending (MIP list)</p>
                  <p><strong>Platform:</strong> All (Linux, macOS, Windows)</p>
                </div>
              )}
              {info.name === 'systemcrypto' && (
                <div className="space-y-2 text-sm text-gray-600">
                  <p>Platform System Crypto delegates to OS-level validated modules via Microsoft Build of Go.</p>
                  <p><strong>Windows:</strong> CNG</p>
                  <p><strong>macOS:</strong> CommonCrypto</p>
                  <p><strong>Linux:</strong> OpenSSL</p>
                </div>
              )}
              {info.name === 'none' && (
                <div className="bg-red-50 border border-red-200 rounded p-3 text-sm text-red-800">
                  <p className="font-medium">No FIPS module detected</p>
                  <p className="mt-1">Rebuild with <code className="bg-red-100 px-1 rounded">GOEXPERIMENT=boringcrypto</code> (Linux) or <code className="bg-red-100 px-1 rounded">GODEBUG=fips140=on</code> (cross-platform).</p>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function getStandardBadge(standard: string): { text: string; color: string } {
  switch (standard) {
    case '140-3':
      return { text: 'FIPS 140-3', color: 'bg-green-100 text-green-800 border-green-200' }
    case '140-2':
      return { text: 'FIPS 140-2', color: 'bg-blue-100 text-blue-800 border-blue-200' }
    case '140-3 (pending)':
      return { text: 'FIPS 140-3 (pending)', color: 'bg-yellow-100 text-yellow-800 border-yellow-200' }
    default:
      return { text: 'No FIPS', color: 'bg-red-100 text-red-800 border-red-200' }
  }
}
