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
            Build {manifest.build_id} &mdash; {manifest.timestamp}
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
                Source
              </h4>
              <dl className="space-y-1 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">Repository</dt>
                  <dd className="text-gray-900 font-mono text-xs">
                    {manifest.source.repository}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Tag</dt>
                  <dd className="text-gray-900 font-mono">{manifest.source.tag}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Commit</dt>
                  <dd className="text-gray-900 font-mono">{manifest.source.commit}</dd>
                </div>
              </dl>
            </div>

            <div>
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Platform
              </h4>
              <dl className="space-y-1 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">Target</dt>
                  <dd className="text-gray-900 font-mono">
                    {manifest.platform.target_os}/{manifest.platform.target_arch}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Builder</dt>
                  <dd className="text-gray-900 font-mono">
                    {manifest.platform.builder}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Go Version</dt>
                  <dd className="text-gray-900 font-mono">
                    {manifest.go_version}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">GOEXPERIMENT</dt>
                  <dd className="text-gray-900 font-mono">
                    {manifest.go_experiment}
                  </dd>
                </div>
              </dl>
            </div>

            <div>
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
                FIPS Certificates
              </h4>
              {manifest.fips_certificates.map((cert) => (
                <div
                  key={cert.cert_number}
                  className="bg-green-50 border border-green-200 rounded-lg p-3 text-sm"
                >
                  <p className="font-semibold text-green-800">
                    {cert.module} &mdash; #{cert.cert_number}
                  </p>
                  <p className="text-green-700">{cert.level}</p>
                  <p className="text-green-600 text-xs mt-1">
                    Validated: {cert.validated_on} | Expires: {cert.expires_on}
                  </p>
                </div>
              ))}
            </div>

            <div>
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Verification
              </h4>
              <dl className="space-y-1 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">BoringCrypto Symbols</dt>
                  <dd>
                    {manifest.verification.boring_crypto_symbols ? (
                      <span className="text-green-600 font-medium">Verified</span>
                    ) : (
                      <span className="text-red-600 font-medium">Missing</span>
                    )}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Self-Test</dt>
                  <dd>
                    {manifest.verification.self_test_passed ? (
                      <span className="text-green-600 font-medium">Passed</span>
                    ) : (
                      <span className="text-red-600 font-medium">Failed</span>
                    )}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">Banned Ciphers</dt>
                  <dd className="text-gray-900">
                    {manifest.verification.banned_ciphers.length === 0
                      ? 'None detected'
                      : manifest.verification.banned_ciphers.join(', ')}
                  </dd>
                </div>
              </dl>
            </div>
          </div>

          <div className="mt-4 pt-4 border-t border-gray-100">
            <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Binary
            </h4>
            <p className="text-xs font-mono text-gray-600 break-all">
              SHA-256: {manifest.binary.sha256}
            </p>
            <p className="text-xs text-gray-500 mt-1">
              Size: {(manifest.binary.size / 1024 / 1024).toFixed(1)} MB
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
