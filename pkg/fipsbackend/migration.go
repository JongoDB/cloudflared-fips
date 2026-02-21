package fipsbackend

import (
	"fmt"
	"time"
)

// FIPS1402SunsetDate is the date when all FIPS 140-2 certificates are archived.
// After this date, only FIPS 140-3 certificates are valid for new procurements.
var FIPS1402SunsetDate = time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)

// MigrationStatus describes the FIPS 140-2 to 140-3 migration state.
type MigrationStatus struct {
	CurrentStandard    string `json:"current_standard"`     // "140-2", "140-3", "140-3 (pending)"
	SunsetDate         string `json:"sunset_date"`          // "2026-09-21"
	DaysUntilSunset    int    `json:"days_until_sunset"`    // negative if past
	MigrationRequired  bool   `json:"migration_required"`   // true if using 140-2
	MigrationUrgency   string `json:"migration_urgency"`    // "none", "low", "medium", "high", "critical"
	RecommendedAction  string `json:"recommended_action"`
	AlternativeBackend string `json:"alternative_backend"`  // recommended 140-3 backend
}

// GetMigrationStatus returns the migration status for the active backend.
func GetMigrationStatus() MigrationStatus {
	backend := Detect()
	now := time.Now()
	daysUntil := int(time.Until(FIPS1402SunsetDate).Hours() / 24)

	status := MigrationStatus{
		SunsetDate:      FIPS1402SunsetDate.Format("2006-01-02"),
		DaysUntilSunset: daysUntil,
	}

	if backend == nil {
		status.CurrentStandard = "none"
		status.MigrationRequired = true
		status.MigrationUrgency = "critical"
		status.RecommendedAction = "Install a FIPS cryptographic module. Use GOEXPERIMENT=boringcrypto (Linux) or GODEBUG=fips140=on (cross-platform)."
		status.AlternativeBackend = "go-native"
		return status
	}

	status.CurrentStandard = backend.FIPSStandard()

	switch backend.FIPSStandard() {
	case "140-2":
		status.MigrationRequired = true
		status.AlternativeBackend = "go-native (FIPS 140-3, once CMVP validated)"

		switch {
		case daysUntil <= 0:
			status.MigrationUrgency = "critical"
			status.RecommendedAction = fmt.Sprintf("FIPS 140-2 sunset has passed (%s). Migrate to FIPS 140-3 immediately. Use BoringCrypto 140-3 (#4735) or Go native FIPS.", FIPS1402SunsetDate.Format("Jan 2, 2006"))
		case daysUntil <= 30:
			status.MigrationUrgency = "critical"
			status.RecommendedAction = fmt.Sprintf("FIPS 140-2 sunset in %d days. Begin migration to FIPS 140-3 immediately.", daysUntil)
		case daysUntil <= 90:
			status.MigrationUrgency = "high"
			status.RecommendedAction = fmt.Sprintf("FIPS 140-2 sunset in %d days. Plan migration to FIPS 140-3.", daysUntil)
		case daysUntil <= 180:
			status.MigrationUrgency = "medium"
			status.RecommendedAction = fmt.Sprintf("FIPS 140-2 sunset in %d days. Test FIPS 140-3 modules in staging.", daysUntil)
		default:
			status.MigrationUrgency = "low"
			status.RecommendedAction = fmt.Sprintf("FIPS 140-2 sunset in %d days. No immediate action required.", daysUntil)
		}

	case "140-3":
		status.MigrationRequired = false
		status.MigrationUrgency = "none"
		status.RecommendedAction = "Already using FIPS 140-3. No migration needed."

	case "140-3 (pending)":
		status.MigrationRequired = false
		status.MigrationUrgency = "low"
		status.RecommendedAction = "Using Go native FIPS 140-3 (CAVP validated, CMVP pending). Monitor CMVP validation status."
		if now.After(FIPS1402SunsetDate) {
			status.MigrationUrgency = "medium"
			status.RecommendedAction = "FIPS 140-2 sunset passed. Go native FIPS is CAVP-validated but CMVP pending. Consider BoringCrypto 140-3 (#4735) for full validation."
		}

	default:
		status.MigrationRequired = true
		status.MigrationUrgency = "high"
		status.RecommendedAction = "Unknown FIPS standard. Verify the active cryptographic module."
	}

	return status
}

// AllBackendMigrationInfo returns migration-relevant info for all known backends.
func AllBackendMigrationInfo() []BackendMigrationInfo {
	return []BackendMigrationInfo{
		{
			Name:           "boringcrypto",
			DisplayName:    "BoringCrypto (BoringSSL)",
			FIPS1402Cert:   "#3678 / #4407",
			FIPS1403Cert:   "#4735",
			Platform:       "Linux amd64/arm64",
			Status1402:     "active (until 2026-09-21)",
			Status1403:     "validated",
			MigrationNotes: "Update to BoringSSL tag certified for 140-3. Same GOEXPERIMENT=boringcrypto build flag.",
		},
		{
			Name:           "go-native",
			DisplayName:    "Go Cryptographic Module",
			FIPS1402Cert:   "n/a",
			FIPS1403Cert:   "CAVP A6650 (CMVP pending)",
			Platform:       "All (Linux, macOS, Windows)",
			Status1402:     "n/a",
			Status1403:     "CAVP validated, CMVP in process",
			MigrationNotes: "Pure Go, no CGO. Cross-platform. Use GODEBUG=fips140=on. Wait for CMVP validation.",
		},
		{
			Name:           "systemcrypto",
			DisplayName:    "Platform System Crypto (Microsoft Go)",
			FIPS1402Cert:   "#4515 (CNG), #3856 (corecrypto)",
			FIPS1403Cert:   "varies by platform",
			Platform:       "Windows (CNG), macOS (corecrypto), Linux (OpenSSL)",
			Status1402:     "active (platform-dependent)",
			Status1403:     "varies (Windows CNG has 140-3 path)",
			MigrationNotes: "Requires Microsoft Build of Go. Uses dlopen to platform crypto. Validation depends on OS vendor.",
		},
	}
}

// BackendMigrationInfo holds migration-relevant details for a FIPS backend.
type BackendMigrationInfo struct {
	Name           string `json:"name"`
	DisplayName    string `json:"display_name"`
	FIPS1402Cert   string `json:"fips_140_2_cert"`
	FIPS1403Cert   string `json:"fips_140_3_cert"`
	Platform       string `json:"platform"`
	Status1402     string `json:"status_140_2"`
	Status1403     string `json:"status_140_3"`
	MigrationNotes string `json:"migration_notes"`
}
