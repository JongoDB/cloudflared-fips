import { useState, useEffect } from 'react'
import Layout from '../components/Layout'
import ControlCoverageTable from '../components/ControlCoverageTable'

interface ComplianceInfo {
  product_name: string
  product_version: string
  description: string
  segments: { name: string; description: string; fips_control: string }[]
  crypto_modules: { name: string; cmvp_cert: string; standard: string; platform: string; build_flag: string; description: string }[]
  controls_covered: { control_id: string; control_name: string; implementation: string; status: string }[]
  verification_methods: { method: string; description: string; example: string }[]
  known_gaps: { gap: string; impact: string; mitigation: string }[]
  deployment_tiers: { tier: string; name: string; description: string }[]
  migration: { sunset_date: string; status: string; plan: string }
  documents: { name: string; path: string; description: string }[]
}

export default function ComplianceInfoPage() {
  const [info, setInfo] = useState<ComplianceInfo | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/v1/compliance-info')
      .then(res => res.ok ? res.json() : null)
      .then(data => { setInfo(data); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-16">
          <div className="w-8 h-8 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
        </div>
      </Layout>
    )
  }

  if (!info) {
    return (
      <Layout>
        <div className="text-center py-16 text-gray-500">
          Unable to load compliance information. Ensure the dashboard backend is running.
        </div>
      </Layout>
    )
  }

  return (
    <Layout>
      <div className="space-y-8">
        {/* Executive Summary */}
        <section>
          <h2 className="text-2xl font-bold text-gray-900 mb-2">AO Compliance Package</h2>
          <p className="text-gray-600 mb-4">{info.description}</p>
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
            <p className="text-sm text-blue-800">
              <strong>Product:</strong> {info.product_name} {info.product_version && `v${info.product_version}`}
              {' | '}
              <strong>Total Controls:</strong> {info.controls_covered.length}
              {' | '}
              <strong>Documents:</strong> {info.documents.length}
            </p>
          </div>
        </section>

        {/* Three-Segment Architecture */}
        <section>
          <h3 className="text-lg font-semibold text-gray-900 mb-3">Three-Segment FIPS Architecture</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {info.segments.map((seg, i) => (
              <div key={i} className="bg-white border border-gray-200 rounded-lg p-4 shadow-sm">
                <h4 className="font-medium text-gray-900 mb-1">{seg.name}</h4>
                <p className="text-sm text-gray-600 mb-2">{seg.description}</p>
                <p className="text-xs text-blue-700 bg-blue-50 rounded px-2 py-1">{seg.fips_control}</p>
              </div>
            ))}
          </div>
        </section>

        {/* Cryptographic Module Inventory */}
        <section>
          <h3 className="text-lg font-semibold text-gray-900 mb-3">FIPS Cryptographic Module Inventory</h3>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Module</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">CMVP Cert</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Standard</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Platform</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Build Flag</th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {info.crypto_modules.map((mod, i) => (
                  <tr key={i} className="hover:bg-gray-50">
                    <td className="px-4 py-2 text-sm font-medium text-gray-900">{mod.name}</td>
                    <td className="px-4 py-2 text-sm font-mono text-blue-700">{mod.cmvp_cert}</td>
                    <td className="px-4 py-2 text-sm">
                      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                        mod.standard.includes('140-3') ? 'bg-green-100 text-green-800' : 'bg-yellow-100 text-yellow-800'
                      }`}>
                        {mod.standard}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-sm text-gray-600">{mod.platform}</td>
                    <td className="px-4 py-2 text-xs font-mono text-gray-500">{mod.build_flag}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        {/* NIST 800-53 Control Coverage */}
        <section>
          <h3 className="text-lg font-semibold text-gray-900 mb-3">NIST SP 800-53 Control Coverage</h3>
          <ControlCoverageTable controls={info.controls_covered.map(c => ({
            controlId: c.control_id,
            controlName: c.control_name,
            implementation: c.implementation,
            status: c.status,
          }))} />
        </section>

        {/* Verification Methods */}
        <section>
          <h3 className="text-lg font-semibold text-gray-900 mb-3">Verification Methods</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {info.verification_methods.map((vm) => {
              const colors: Record<string, string> = {
                direct: 'bg-green-100 text-green-800 border-green-200',
                api: 'bg-blue-100 text-blue-800 border-blue-200',
                probe: 'bg-purple-100 text-purple-800 border-purple-200',
                inherited: 'bg-yellow-100 text-yellow-800 border-yellow-200',
                reported: 'bg-gray-100 text-gray-800 border-gray-200',
              }
              return (
                <div key={vm.method} className={`rounded-lg border p-3 ${colors[vm.method] || 'bg-gray-100'}`}>
                  <span className="text-xs font-bold uppercase">{vm.method}</span>
                  <p className="text-sm mt-1">{vm.description}</p>
                  <p className="text-xs mt-1 opacity-75">Example: {vm.example}</p>
                </div>
              )
            })}
          </div>
        </section>

        {/* Known Gaps & Mitigations */}
        <section>
          <h3 className="text-lg font-semibold text-gray-900 mb-3">Known Gaps & Mitigations</h3>
          <div className="space-y-3">
            {info.known_gaps.map((gap, i) => (
              <div key={i} className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
                <p className="text-sm font-medium text-yellow-900">{gap.gap}</p>
                <p className="text-sm text-yellow-800 mt-1"><strong>Impact:</strong> {gap.impact}</p>
                <p className="text-sm text-yellow-700 mt-1"><strong>Mitigation:</strong> {gap.mitigation}</p>
              </div>
            ))}
          </div>
        </section>

        {/* Deployment Tiers */}
        <section>
          <h3 className="text-lg font-semibold text-gray-900 mb-3">Deployment Tiers</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {info.deployment_tiers.map((tier) => (
              <div key={tier.tier} className="bg-white border border-gray-200 rounded-lg p-4 shadow-sm">
                <div className="flex items-center gap-2 mb-2">
                  <span className="inline-flex items-center justify-center w-8 h-8 rounded-full bg-blue-100 text-blue-800 text-sm font-bold">
                    {tier.tier}
                  </span>
                  <h4 className="font-medium text-gray-900">{tier.name}</h4>
                </div>
                <p className="text-sm text-gray-600">{tier.description}</p>
              </div>
            ))}
          </div>
        </section>

        {/* FIPS 140-2 to 140-3 Migration */}
        <section>
          <h3 className="text-lg font-semibold text-gray-900 mb-3">FIPS 140-2 to 140-3 Migration</h3>
          <div className="bg-orange-50 border border-orange-200 rounded-lg p-4">
            <p className="text-sm text-orange-900"><strong>Sunset Date:</strong> {info.migration.sunset_date}</p>
            <p className="text-sm text-orange-800 mt-1"><strong>Current Status:</strong> {info.migration.status}</p>
            <p className="text-sm text-orange-700 mt-1"><strong>Migration Plan:</strong> {info.migration.plan}</p>
          </div>
        </section>

        {/* Documentation Index */}
        <section>
          <h3 className="text-lg font-semibold text-gray-900 mb-3">AO Documentation Package</h3>
          <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
            <div className="divide-y divide-gray-200">
              {info.documents.map((doc) => (
                <div key={doc.path} className="px-4 py-3 hover:bg-gray-50">
                  <p className="text-sm font-medium text-gray-900">{doc.name}</p>
                  <p className="text-xs text-gray-500 mt-0.5">{doc.description}</p>
                  <p className="text-xs font-mono text-blue-600 mt-0.5">{doc.path}</p>
                </div>
              ))}
            </div>
          </div>
        </section>
      </div>
    </Layout>
  )
}
