import { useState, useEffect } from 'react'

export interface MigrationStatus {
  current_standard: string
  sunset_date: string
  days_until_sunset: number
  migration_required: boolean
  migration_urgency: string
  recommended_action: string
  alternative_backend: string
}

const defaultMigration: MigrationStatus = {
  current_standard: '140-2',
  sunset_date: '2026-09-21',
  days_until_sunset: Math.ceil(
    (new Date('2026-09-21').getTime() - Date.now()) / (1000 * 60 * 60 * 24)
  ),
  migration_required: true,
  migration_urgency: 'medium',
  recommended_action:
    'FIPS 140-2 sunset approaching. Test FIPS 140-3 modules (BoringCrypto #4735 or Go native) in staging.',
  alternative_backend: 'go-native',
}

export function useComplianceMigration(url = '/api/v1/migration') {
  const [migration, setMigration] = useState<MigrationStatus>(defaultMigration)

  useEffect(() => {
    fetch(url)
      .then((res) => {
        if (res.ok) return res.json()
        throw new Error('API unavailable')
      })
      .then((data: MigrationStatus) => setMigration(data))
      .catch(() => {
        // Keep default mock data
      })
  }, [url])

  return migration
}
