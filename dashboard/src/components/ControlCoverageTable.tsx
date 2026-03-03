interface ControlRow {
  controlId: string
  controlName: string
  implementation: string
  status: string
}

interface ControlCoverageTableProps {
  controls: ControlRow[]
}

export default function ControlCoverageTable({ controls }: ControlCoverageTableProps) {
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Control</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Implementation</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-gray-200">
          {controls.map((c) => (
            <tr key={c.controlId} className="hover:bg-gray-50">
              <td className="px-4 py-2 whitespace-nowrap text-sm font-mono text-blue-700">{c.controlId}</td>
              <td className="px-4 py-2 text-sm text-gray-900">{c.controlName}</td>
              <td className="px-4 py-2 text-sm text-gray-600">{c.implementation}</td>
              <td className="px-4 py-2 whitespace-nowrap">
                <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                  c.status === 'implemented' ? 'bg-green-100 text-green-800' :
                  c.status === 'partial' ? 'bg-yellow-100 text-yellow-800' :
                  'bg-gray-100 text-gray-800'
                }`}>
                  {c.status}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
