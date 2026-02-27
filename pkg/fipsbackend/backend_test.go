package fipsbackend

import (
	"runtime"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// AllBackends
// ---------------------------------------------------------------------------

func TestAllBackendsReturnsThreeBackends(t *testing.T) {
	backends := AllBackends()
	if len(backends) != 3 {
		t.Fatalf("AllBackends() returned %d backends, want 3", len(backends))
	}
}

func TestAllBackendsOrder(t *testing.T) {
	backends := AllBackends()
	wantNames := []string{"boringcrypto", "go-native", "systemcrypto"}
	for i, want := range wantNames {
		got := backends[i].Name()
		if got != want {
			t.Errorf("AllBackends()[%d].Name() = %q, want %q", i, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// BoringCrypto identity
// ---------------------------------------------------------------------------

func TestBoringCryptoName(t *testing.T) {
	b := &BoringCrypto{}
	if got := b.Name(); got != "boringcrypto" {
		t.Errorf("BoringCrypto.Name() = %q, want %q", got, "boringcrypto")
	}
}

func TestBoringCryptoDisplayName(t *testing.T) {
	b := &BoringCrypto{}
	if got := b.DisplayName(); got != "BoringCrypto (BoringSSL)" {
		t.Errorf("BoringCrypto.DisplayName() = %q, want %q", got, "BoringCrypto (BoringSSL)")
	}
}

func TestBoringCryptoCMVPCertificate(t *testing.T) {
	b := &BoringCrypto{}
	if got := b.CMVPCertificate(); got != "#4407 (140-2), #4735 (140-3)" {
		t.Errorf("BoringCrypto.CMVPCertificate() = %q, want %q", got, "#4407 (140-2), #4735 (140-3)")
	}
}

func TestBoringCryptoValidated(t *testing.T) {
	b := &BoringCrypto{}
	if !b.Validated() {
		t.Error("BoringCrypto.Validated() = false, want true")
	}
}

// ---------------------------------------------------------------------------
// BoringCrypto.FIPSStandard (tests goVersionAtLeast indirectly)
// ---------------------------------------------------------------------------

func TestBoringCryptoFIPSStandard(t *testing.T) {
	b := &BoringCrypto{}
	got := b.FIPSStandard()
	// We are running Go 1.24+, so FIPSStandard should return "140-3".
	if !goVersionAtLeast(1, 24) {
		t.Skipf("Test requires Go >= 1.24, running %s", runtime.Version())
	}
	if got != "140-3" {
		t.Errorf("BoringCrypto.FIPSStandard() = %q on Go %s, want %q", got, runtime.Version(), "140-3")
	}
}

// goVersionAtLeast is tested indirectly through FIPSStandard, but we can also
// verify known invariants about the running Go version.
func TestGoVersionAtLeastCurrentVersion(t *testing.T) {
	// We know runtime.Version() is at least "go1.24" because go.mod requires it.
	if !goVersionAtLeast(1, 24) {
		t.Errorf("goVersionAtLeast(1, 24) = false, but runtime.Version() = %q", runtime.Version())
	}
	// A version below ours must be true.
	if !goVersionAtLeast(1, 20) {
		t.Errorf("goVersionAtLeast(1, 20) = false, but runtime.Version() = %q", runtime.Version())
	}
	if !goVersionAtLeast(1, 1) {
		t.Errorf("goVersionAtLeast(1, 1) = false, but runtime.Version() = %q", runtime.Version())
	}
	// A future version must be false.
	if goVersionAtLeast(2, 0) {
		t.Errorf("goVersionAtLeast(2, 0) = true, but runtime.Version() = %q", runtime.Version())
	}
	if goVersionAtLeast(1, 99) {
		t.Errorf("goVersionAtLeast(1, 99) = true, but runtime.Version() = %q", runtime.Version())
	}
}

// ---------------------------------------------------------------------------
// BoringCrypto.Active — depends on build; in a standard build it is false
// ---------------------------------------------------------------------------

func TestBoringCryptoActiveStandardBuild(t *testing.T) {
	b := &BoringCrypto{}
	// In a standard (non-boringcrypto) build the cipher heuristic returns false.
	// We cannot control this from a test, but we can verify it is consistent
	// with isBoringCryptoActive().
	want := isBoringCryptoActive()
	got := b.Active()
	if got != want {
		t.Errorf("BoringCrypto.Active() = %v, want %v (matching isBoringCryptoActive)", got, want)
	}
}

// ---------------------------------------------------------------------------
// BoringCrypto.SelfTest
// ---------------------------------------------------------------------------

func TestBoringCryptoSelfTestWhenInactive(t *testing.T) {
	b := &BoringCrypto{}
	if b.Active() {
		t.Skip("BoringCrypto is active in this build; cannot test inactive path")
	}
	ok, err := b.SelfTest()
	if ok {
		t.Error("BoringCrypto.SelfTest() returned true when inactive")
	}
	if err != nil {
		t.Errorf("BoringCrypto.SelfTest() returned unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GoNative identity
// ---------------------------------------------------------------------------

func TestGoNativeName(t *testing.T) {
	g := &GoNative{}
	if got := g.Name(); got != "go-native" {
		t.Errorf("GoNative.Name() = %q, want %q", got, "go-native")
	}
}

func TestGoNativeDisplayName(t *testing.T) {
	g := &GoNative{}
	if got := g.DisplayName(); got != "Go Cryptographic Module (native)" {
		t.Errorf("GoNative.DisplayName() = %q, want %q", got, "Go Cryptographic Module (native)")
	}
}

func TestGoNativeCMVPCertificate(t *testing.T) {
	g := &GoNative{}
	if got := g.CMVPCertificate(); got != GoNativeCMVPCert {
		t.Errorf("GoNative.CMVPCertificate() = %q, want %q", got, GoNativeCMVPCert)
	}
}

// ---------------------------------------------------------------------------
// GoNative.Active with GODEBUG manipulation
// ---------------------------------------------------------------------------

func TestGoNativeActiveWithFIPS140On(t *testing.T) {
	t.Setenv("GODEBUG", "fips140=on")
	g := &GoNative{}
	if !g.Active() {
		t.Error("GoNative.Active() = false with GODEBUG=fips140=on, want true")
	}
}

func TestGoNativeActiveWithFIPS140Only(t *testing.T) {
	t.Setenv("GODEBUG", "fips140=only")
	g := &GoNative{}
	if !g.Active() {
		t.Error("GoNative.Active() = false with GODEBUG=fips140=only, want true")
	}
}

func TestGoNativeActiveWithMultipleGODEBUGEntries(t *testing.T) {
	t.Setenv("GODEBUG", "http2debug=1,fips140=on,netdns=go")
	g := &GoNative{}
	if !g.Active() {
		t.Error("GoNative.Active() = false with GODEBUG containing fips140=on among other entries, want true")
	}
}

func TestGoNativeInactiveWithoutGODEBUG(t *testing.T) {
	t.Setenv("GODEBUG", "")
	g := &GoNative{}
	if g.Active() {
		t.Error("GoNative.Active() = true with empty GODEBUG, want false")
	}
}

func TestGoNativeInactiveWithUnrelatedGODEBUG(t *testing.T) {
	t.Setenv("GODEBUG", "http2debug=1,netdns=go")
	g := &GoNative{}
	if g.Active() {
		t.Error("GoNative.Active() = true with GODEBUG not containing fips140, want false")
	}
}

func TestGoNativeInactiveWithFIPS140Off(t *testing.T) {
	t.Setenv("GODEBUG", "fips140=off")
	g := &GoNative{}
	if g.Active() {
		t.Error("GoNative.Active() = true with GODEBUG=fips140=off, want false")
	}
}

// ---------------------------------------------------------------------------
// GoNative.FIPSStandard and Validated (uses package-level vars)
// ---------------------------------------------------------------------------

func TestGoNativeFIPSStandardPending(t *testing.T) {
	origValidated := GoNativeCMVPValidated
	t.Cleanup(func() { GoNativeCMVPValidated = origValidated })

	GoNativeCMVPValidated = false
	g := &GoNative{}
	if got := g.FIPSStandard(); got != "140-3 (pending)" {
		t.Errorf("GoNative.FIPSStandard() = %q when not validated, want %q", got, "140-3 (pending)")
	}
	if g.Validated() {
		t.Error("GoNative.Validated() = true when GoNativeCMVPValidated is false")
	}
}

func TestGoNativeFIPSStandardValidated(t *testing.T) {
	origValidated := GoNativeCMVPValidated
	origCert := GoNativeCMVPCert
	t.Cleanup(func() {
		GoNativeCMVPValidated = origValidated
		GoNativeCMVPCert = origCert
	})

	GoNativeCMVPValidated = true
	GoNativeCMVPCert = "#99999"
	g := &GoNative{}
	if got := g.FIPSStandard(); got != "140-3" {
		t.Errorf("GoNative.FIPSStandard() = %q when validated, want %q", got, "140-3")
	}
	if !g.Validated() {
		t.Error("GoNative.Validated() = false when GoNativeCMVPValidated is true")
	}
	if got := g.CMVPCertificate(); got != "#99999" {
		t.Errorf("GoNative.CMVPCertificate() = %q, want %q", got, "#99999")
	}
}

// ---------------------------------------------------------------------------
// GoNative.SelfTest
// ---------------------------------------------------------------------------

func TestGoNativeSelfTestWhenActive(t *testing.T) {
	t.Setenv("GODEBUG", "fips140=on")
	g := &GoNative{}
	ok, err := g.SelfTest()
	if !ok {
		t.Error("GoNative.SelfTest() returned false when active")
	}
	if err != nil {
		t.Errorf("GoNative.SelfTest() returned unexpected error: %v", err)
	}
}

func TestGoNativeSelfTestWhenInactive(t *testing.T) {
	t.Setenv("GODEBUG", "")
	g := &GoNative{}
	ok, err := g.SelfTest()
	if ok {
		t.Error("GoNative.SelfTest() returned true when inactive")
	}
	if err != nil {
		t.Errorf("GoNative.SelfTest() returned unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SystemCrypto
// ---------------------------------------------------------------------------

func TestSystemCryptoName(t *testing.T) {
	s := &SystemCrypto{}
	if got := s.Name(); got != "systemcrypto" {
		t.Errorf("SystemCrypto.Name() = %q, want %q", got, "systemcrypto")
	}
}

func TestSystemCryptoDisplayName(t *testing.T) {
	s := &SystemCrypto{}
	if got := s.DisplayName(); got != "Platform System Crypto" {
		t.Errorf("SystemCrypto.DisplayName() = %q, want %q", got, "Platform System Crypto")
	}
}

func TestSystemCryptoActiveAlwaysFalse(t *testing.T) {
	s := &SystemCrypto{}
	if s.Active() {
		t.Error("SystemCrypto.Active() = true, want false (stub)")
	}
}

func TestSystemCryptoSelfTestWhenInactive(t *testing.T) {
	s := &SystemCrypto{}
	ok, err := s.SelfTest()
	if ok {
		t.Error("SystemCrypto.SelfTest() returned true when inactive")
	}
	if err != nil {
		t.Errorf("SystemCrypto.SelfTest() returned unexpected error: %v", err)
	}
}

func TestSystemCryptoCMVPCertificateLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("Test requires linux, running on %s", runtime.GOOS)
	}
	s := &SystemCrypto{}
	if got := s.CMVPCertificate(); got != "Varies by distro OpenSSL" {
		t.Errorf("SystemCrypto.CMVPCertificate() on linux = %q, want %q", got, "Varies by distro OpenSSL")
	}
}

func TestSystemCryptoFIPSStandardLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("Test requires linux, running on %s", runtime.GOOS)
	}
	s := &SystemCrypto{}
	if got := s.FIPSStandard(); got != "varies" {
		t.Errorf("SystemCrypto.FIPSStandard() on linux = %q, want %q", got, "varies")
	}
}

func TestSystemCryptoValidatedLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("Test requires linux, running on %s", runtime.GOOS)
	}
	s := &SystemCrypto{}
	// On Linux, Validated returns false (depends on distro).
	if s.Validated() {
		t.Error("SystemCrypto.Validated() on linux = true, want false")
	}
}

// ---------------------------------------------------------------------------
// splitComma
// ---------------------------------------------------------------------------

func TestSplitComma(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"fips140=on", []string{"fips140=on"}},
		{"http2debug=1,fips140=on,netdns=go", []string{"http2debug=1", "fips140=on", "netdns=go"}},
		// Consecutive commas produce no empty strings (implementation skips empty)
		{"a,,b", []string{"a", "b"}},
		{",a,", []string{"a"}},
		{",", nil},
		{",,", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitComma(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitComma(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitComma(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ToInfo
// ---------------------------------------------------------------------------

func TestToInfo(t *testing.T) {
	b := &BoringCrypto{}
	info := ToInfo(b)
	if info.Name != "boringcrypto" {
		t.Errorf("ToInfo().Name = %q, want %q", info.Name, "boringcrypto")
	}
	if info.DisplayName != "BoringCrypto (BoringSSL)" {
		t.Errorf("ToInfo().DisplayName = %q, want %q", info.DisplayName, "BoringCrypto (BoringSSL)")
	}
	if info.CMVPCertificate != "#4407 (140-2), #4735 (140-3)" {
		t.Errorf("ToInfo().CMVPCertificate = %q, want %q", info.CMVPCertificate, "#4407 (140-2), #4735 (140-3)")
	}
	if info.Validated != true {
		t.Error("ToInfo().Validated = false, want true")
	}
	if info.Active != b.Active() {
		t.Errorf("ToInfo().Active = %v, want %v", info.Active, b.Active())
	}
}

func TestToInfoGoNative(t *testing.T) {
	t.Setenv("GODEBUG", "fips140=on")
	g := &GoNative{}
	info := ToInfo(g)
	if info.Name != "go-native" {
		t.Errorf("ToInfo(GoNative).Name = %q, want %q", info.Name, "go-native")
	}
	if !info.Active {
		t.Error("ToInfo(GoNative).Active = false with GODEBUG=fips140=on, want true")
	}
}

// ---------------------------------------------------------------------------
// Detect / DetectInfo
// ---------------------------------------------------------------------------

func TestDetectReturnsNilWhenNoBackendActive(t *testing.T) {
	// Ensure GODEBUG does not activate GoNative.
	t.Setenv("GODEBUG", "")
	b := Detect()
	// In a standard (non-boring) build with no GODEBUG, nothing is active.
	if isBoringCryptoActive() {
		t.Skip("BoringCrypto is active in this build; cannot test nil Detect path")
	}
	if b != nil {
		t.Errorf("Detect() = %v (name=%q), want nil", b, b.Name())
	}
}

func TestDetectInfoReturnsNonePlaceholder(t *testing.T) {
	t.Setenv("GODEBUG", "")
	if isBoringCryptoActive() {
		t.Skip("BoringCrypto is active in this build; cannot test none placeholder")
	}
	info := DetectInfo()
	if info.Name != "none" {
		t.Errorf("DetectInfo().Name = %q, want %q", info.Name, "none")
	}
	if info.DisplayName != "No FIPS Module" {
		t.Errorf("DetectInfo().DisplayName = %q, want %q", info.DisplayName, "No FIPS Module")
	}
	if info.CMVPCertificate != "n/a" {
		t.Errorf("DetectInfo().CMVPCertificate = %q, want %q", info.CMVPCertificate, "n/a")
	}
	if info.FIPSStandard != "n/a" {
		t.Errorf("DetectInfo().FIPSStandard = %q, want %q", info.FIPSStandard, "n/a")
	}
	if info.Validated {
		t.Error("DetectInfo().Validated = true, want false")
	}
	if info.Active {
		t.Error("DetectInfo().Active = true, want false")
	}
}

func TestDetectReturnsGoNativeWhenGODEBUGSet(t *testing.T) {
	t.Setenv("GODEBUG", "fips140=on")
	b := Detect()
	if b == nil {
		t.Fatal("Detect() = nil with GODEBUG=fips140=on, want GoNative")
	}
	if b.Name() != "go-native" {
		t.Errorf("Detect().Name() = %q, want %q", b.Name(), "go-native")
	}
}

func TestDetectInfoReturnsGoNativeWhenGODEBUGSet(t *testing.T) {
	t.Setenv("GODEBUG", "fips140=on")
	info := DetectInfo()
	if info.Name != "go-native" {
		t.Errorf("DetectInfo().Name = %q with GODEBUG=fips140=on, want %q", info.Name, "go-native")
	}
	if !info.Active {
		t.Error("DetectInfo().Active = false, want true")
	}
}

// If BoringCrypto is detected as active (build-dependent), it comes before GoNative
// in AllBackends, so Detect should return BoringCrypto even if GODEBUG is also set.
func TestDetectPrefersBoringCryptoOverGoNative(t *testing.T) {
	if !isBoringCryptoActive() {
		t.Skip("BoringCrypto not active in this build")
	}
	t.Setenv("GODEBUG", "fips140=on")
	b := Detect()
	if b == nil {
		t.Fatal("Detect() = nil, want non-nil")
	}
	if b.Name() != "boringcrypto" {
		t.Errorf("Detect().Name() = %q, want %q (BoringCrypto before GoNative)", b.Name(), "boringcrypto")
	}
}

// ---------------------------------------------------------------------------
// FIPS 140-2 Sunset Date
// ---------------------------------------------------------------------------

func TestFIPS1402SunsetDate(t *testing.T) {
	expected := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	if !FIPS1402SunsetDate.Equal(expected) {
		t.Errorf("FIPS1402SunsetDate = %v, want %v", FIPS1402SunsetDate, expected)
	}
}

// ---------------------------------------------------------------------------
// MigrationStatus urgency levels
// ---------------------------------------------------------------------------

func TestGetMigrationStatusNoBackend(t *testing.T) {
	t.Setenv("GODEBUG", "")
	if isBoringCryptoActive() {
		t.Skip("BoringCrypto is active in this build")
	}
	status := GetMigrationStatus()
	if status.CurrentStandard != "none" {
		t.Errorf("MigrationStatus.CurrentStandard = %q, want %q", status.CurrentStandard, "none")
	}
	if !status.MigrationRequired {
		t.Error("MigrationStatus.MigrationRequired = false, want true when no backend")
	}
	if status.MigrationUrgency != "critical" {
		t.Errorf("MigrationStatus.MigrationUrgency = %q, want %q", status.MigrationUrgency, "critical")
	}
	if status.AlternativeBackend != "go-native" {
		t.Errorf("MigrationStatus.AlternativeBackend = %q, want %q", status.AlternativeBackend, "go-native")
	}
}

func TestGetMigrationStatusGoNativePending(t *testing.T) {
	origValidated := GoNativeCMVPValidated
	t.Cleanup(func() { GoNativeCMVPValidated = origValidated })
	GoNativeCMVPValidated = false

	t.Setenv("GODEBUG", "fips140=on")
	status := GetMigrationStatus()
	if status.CurrentStandard != "140-3 (pending)" {
		t.Errorf("MigrationStatus.CurrentStandard = %q, want %q", status.CurrentStandard, "140-3 (pending)")
	}
	if status.MigrationRequired {
		t.Error("MigrationStatus.MigrationRequired = true for 140-3 (pending), want false")
	}
	// Urgency depends on whether we are past sunset date
	if time.Now().Before(FIPS1402SunsetDate) {
		if status.MigrationUrgency != "low" {
			t.Errorf("MigrationStatus.MigrationUrgency = %q, want %q (before sunset)", status.MigrationUrgency, "low")
		}
	} else {
		if status.MigrationUrgency != "medium" {
			t.Errorf("MigrationStatus.MigrationUrgency = %q, want %q (after sunset)", status.MigrationUrgency, "medium")
		}
	}
}

func TestGetMigrationStatusGoNativeValidated(t *testing.T) {
	origValidated := GoNativeCMVPValidated
	t.Cleanup(func() { GoNativeCMVPValidated = origValidated })
	GoNativeCMVPValidated = true

	t.Setenv("GODEBUG", "fips140=on")
	status := GetMigrationStatus()
	if status.CurrentStandard != "140-3" {
		t.Errorf("MigrationStatus.CurrentStandard = %q, want %q", status.CurrentStandard, "140-3")
	}
	if status.MigrationRequired {
		t.Error("MigrationStatus.MigrationRequired = true for validated 140-3, want false")
	}
	if status.MigrationUrgency != "none" {
		t.Errorf("MigrationStatus.MigrationUrgency = %q, want %q", status.MigrationUrgency, "none")
	}
}

func TestGetMigrationStatusSunsetDateField(t *testing.T) {
	t.Setenv("GODEBUG", "")
	status := GetMigrationStatus()
	if status.SunsetDate != "2026-09-21" {
		t.Errorf("MigrationStatus.SunsetDate = %q, want %q", status.SunsetDate, "2026-09-21")
	}
}

func TestGetMigrationStatusDaysUntilSunset(t *testing.T) {
	t.Setenv("GODEBUG", "")
	status := GetMigrationStatus()
	expectedDays := int(time.Until(FIPS1402SunsetDate).Hours() / 24)
	// Allow 1 day tolerance for test runs near midnight.
	diff := status.DaysUntilSunset - expectedDays
	if diff < -1 || diff > 1 {
		t.Errorf("MigrationStatus.DaysUntilSunset = %d, expected approximately %d", status.DaysUntilSunset, expectedDays)
	}
}

// ---------------------------------------------------------------------------
// AllBackendMigrationInfo
// ---------------------------------------------------------------------------

func TestAllBackendMigrationInfoReturnsThreeEntries(t *testing.T) {
	infos := AllBackendMigrationInfo()
	if len(infos) != 3 {
		t.Fatalf("AllBackendMigrationInfo() returned %d entries, want 3", len(infos))
	}
}

func TestAllBackendMigrationInfoNames(t *testing.T) {
	infos := AllBackendMigrationInfo()
	wantNames := []string{"boringcrypto", "go-native", "systemcrypto"}
	for i, want := range wantNames {
		if infos[i].Name != want {
			t.Errorf("AllBackendMigrationInfo()[%d].Name = %q, want %q", i, infos[i].Name, want)
		}
	}
}

func TestAllBackendMigrationInfoBoringCrypto(t *testing.T) {
	infos := AllBackendMigrationInfo()
	bc := infos[0]
	if bc.FIPS1402Cert != "#3678 / #4407" {
		t.Errorf("BoringCrypto migration FIPS1402Cert = %q, want %q", bc.FIPS1402Cert, "#3678 / #4407")
	}
	if bc.FIPS1403Cert != "#4735" {
		t.Errorf("BoringCrypto migration FIPS1403Cert = %q, want %q", bc.FIPS1403Cert, "#4735")
	}
	if bc.Platform != "Linux amd64/arm64" {
		t.Errorf("BoringCrypto migration Platform = %q, want %q", bc.Platform, "Linux amd64/arm64")
	}
}

func TestAllBackendMigrationInfoGoNativeCert(t *testing.T) {
	infos := AllBackendMigrationInfo()
	gn := infos[1]
	if gn.FIPS1403Cert != GoNativeCMVPCert {
		t.Errorf("GoNative migration FIPS1403Cert = %q, want %q", gn.FIPS1403Cert, GoNativeCMVPCert)
	}
}

func TestAllBackendMigrationInfoGoNativeStatus(t *testing.T) {
	origValidated := GoNativeCMVPValidated
	t.Cleanup(func() { GoNativeCMVPValidated = origValidated })

	GoNativeCMVPValidated = false
	infos := AllBackendMigrationInfo()
	gn := infos[1]
	if gn.Status1403 != "CAVP validated, CMVP in process" {
		t.Errorf("GoNative migration Status1403 = %q, want %q", gn.Status1403, "CAVP validated, CMVP in process")
	}

	GoNativeCMVPValidated = true
	infos = AllBackendMigrationInfo()
	gn = infos[1]
	if gn.Status1403 != "CMVP validated" {
		t.Errorf("GoNative migration Status1403 (validated) = %q, want %q", gn.Status1403, "CMVP validated")
	}
}

func TestAllBackendMigrationInfoSystemCrypto(t *testing.T) {
	infos := AllBackendMigrationInfo()
	sc := infos[2]
	if sc.DisplayName != "Platform System Crypto (Microsoft Go)" {
		t.Errorf("SystemCrypto migration DisplayName = %q, want %q", sc.DisplayName, "Platform System Crypto (Microsoft Go)")
	}
}

// ---------------------------------------------------------------------------
// BackendMigrationInfo struct fields (sanity check)
// ---------------------------------------------------------------------------

func TestBackendMigrationInfoFieldsNonEmpty(t *testing.T) {
	for _, info := range AllBackendMigrationInfo() {
		if info.Name == "" {
			t.Error("BackendMigrationInfo has empty Name")
		}
		if info.DisplayName == "" {
			t.Errorf("BackendMigrationInfo %q has empty DisplayName", info.Name)
		}
		if info.Platform == "" {
			t.Errorf("BackendMigrationInfo %q has empty Platform", info.Name)
		}
		if info.MigrationNotes == "" {
			t.Errorf("BackendMigrationInfo %q has empty MigrationNotes", info.Name)
		}
	}
}
