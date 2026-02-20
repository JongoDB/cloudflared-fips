import { useState } from 'react'
import type { BuildManifest } from '../types/compliance'

interface BuildManifestPanelProps {
  manifest: BuildManifest
}

export default function BuildManifestPanel({ manifest }: BuildManifestPanelProps) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="mb-8 bg-white rounded-lg border border-gray-200 shadow-sm">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-6 py-4 flex items-center justify-between text-left hover:bg-gray-50 transition-colors"
      >
        <div>
          <h3 className="text-base font-semibold text-gray-900">
            Build Manifest
          </h3>
          <p className="text-sm text-gray-500">
            v{manifest.version} &mdash; {manifest.build_time}
          </p>
        </div>
        <svg
          className={`w-5 h-5 text-gray-400 transition-transform ${
            expanded ? 'rotate-180' : ''
          }`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>
      {expanded && (
        <div className="px-6 pb-6 border-t border-gray-100">
          <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Build Info
              </h4>
              <dl className="space-y-1 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">Version</dt>
                  <dd className="text-gray-900 font-mono">{manifest.version}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Commit</dt>
                  <dd className="text-gray-900 font-mono">{manifest.commit}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Build Time</dt>
                  <dd className="text-gray-900 font-mono">{manifest.build_time}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Platform</dt>
                  <dd className="text-gray-900 font-mono">{manifest.target_platform}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Package</dt>
                  <dd className="text-gray-900 font-mono">{manifest.package_format}</dd>
                </div>
              </dl>
            </div>

            <div>
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Upstream cloudflared
              </h4>
              <dl className="space-y-1 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">Version</dt>
                  <dd className="text-gray-900 font-mono">
                    {manifest.cloudflared_upstream_version}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Commit</dt>
                  <dd className="text-gray-900 font-mono">
                    {manifest.cloudflared_upstream_commit}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Crypto Engine</dt>
                  <dd className="text-gray-900 font-mono">{manifest.crypto_engine}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">BoringSSL Version</dt>
                  <dd className="text-gray-900 font-mono">{manifest.boringssl_version}</dd>
                </div>
              </dl>
            </div>

            <div className="md:col-span-2">
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
                FIPS Certificates
              </h4>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {manifest.fips_certificates.map((cert) => (
                  <div
                    key={cert.certificate}
                    className="bg-green-50 border border-green-200 rounded-lg p-3 text-sm"
                  >
                    <p className="font-semibold text-green-800">
                      {cert.module} &mdash; {cert.certificate}
                    </p>
                    <p className="text-green-700 text-xs mt-1">
                      Algorithms: {cert.algorithms.slice(0, 6).join(', ')}
                      {cert.algorithms.length > 6 && ` +${cert.algorithms.length - 6} more`}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="mt-4 pt-4 border-t border-gray-100">
            <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Integrity Hashes
            </h4>
            <dl className="space-y-1">
              <div>
                <dt className="text-xs text-gray-500">Binary SHA-256</dt>
                <dd className="text-xs font-mono text-gray-600 break-all">
                  {manifest.binary_sha256}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-gray-500">SBOM SHA-256</dt>
                <dd className="text-xs font-mono text-gray-600 break-all">
                  {manifest.sbom_sha256}
                </dd>
              </div>
            </dl>
          </div>
        </div>
      )}
    </div>
  )
}
