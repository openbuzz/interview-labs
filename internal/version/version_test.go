package version

import "testing"

// Dev builds carry no ldflags pins: version resolves to the edge channel
// and ResolvePins returns the tag-fallback warning.
func TestResolvePinsDevDefaults(t *testing.T) {
	info, warn := ResolvePins()

	if info.Version != "edge" {
		t.Errorf("Version = %q, want edge", info.Version)
	}
	if warn == "" {
		t.Error("dev build must return a pin warning")
	}
	if info.GatewayDigest != "" || len(info.VscodeDigests) != 0 {
		t.Errorf("dev build must have no digests: %+v", info)
	}
}

// Release builds inject all pins via -ldflags -X; the warning is empty and
// the digests parse into the per-profile map.
func TestResolvePinsRelease(t *testing.T) {
	restore := setPins(t, "1.2.0", "abc123", "sha256:gw",
		"devops=sha256:d,backend-ai=sha256:b")
	defer restore()

	info, warn := ResolvePins()

	if warn != "" {
		t.Errorf("release build must not warn: %q", warn)
	}
	if info.Version != "1.2.0" || info.Commit != "abc123" ||
		info.GatewayDigest != "sha256:gw" {
		t.Errorf("info = %+v", info)
	}
	if info.VscodeDigests["devops"] != "sha256:d" ||
		info.VscodeDigests["backend-ai"] != "sha256:b" {
		t.Errorf("vscode digests = %+v", info.VscodeDigests)
	}
}

// A snapshot build stamps a version but never collects digests; the digests
// themselves are the release signal, so it still warns and falls back.
func TestResolvePinsSnapshotWithoutDigestsWarns(t *testing.T) {
	restore := setPins(t, "0.0.0-SNAPSHOT-abc", "abc", "", "")
	defer restore()

	info, warn := ResolvePins()

	if warn == "" {
		t.Error("pin-less snapshot must warn")
	}
	if info.Version != "0.0.0-SNAPSHOT-abc" {
		t.Errorf("Version = %q", info.Version)
	}
}

// go install stamps v-prefixed module versions (v0.1.0); CI image tags are
// stripped by metadata-action (0.1.0), so the resolved version must lose the
// prefix or every pull resolves a nonexistent tag.
func TestResolvePinsGoInstallStripsV(t *testing.T) {
	restore := setPins(t, "dev", "", "", "")
	defer restore()
	oldInstalled := installedVersion
	installedVersion = func() (string, bool) { return "v0.1.0", true }
	defer func() { installedVersion = oldInstalled }()

	info, warn := ResolvePins()

	if info.Version != "0.1.0" {
		t.Errorf("Version = %q, want 0.1.0 (v stripped)", info.Version)
	}
	if warn == "" {
		t.Error("go-install build still resolves by tag and must warn")
	}
}

func TestParseVscodeDigestsSkipsMalformed(t *testing.T) {
	got := parseVscodeDigests("devops=sha256:d,junk,backend=sha256:b")
	if len(got) != 2 || got["devops"] != "sha256:d" || got["backend"] != "sha256:b" {
		t.Errorf("parsed = %+v", got)
	}
	if len(parseVscodeDigests("")) != 0 {
		t.Error("empty input must parse to empty map")
	}
}

// setPins swaps the package vars for one test.
func setPins(t *testing.T, v, c, gw, vs string) func() {
	t.Helper()
	oldV, oldC, oldGW, oldVS := buildVersion, commit, gatewayDigest, vscodeDigests
	buildVersion, commit, gatewayDigest, vscodeDigests = v, c, gw, vs
	return func() {
		buildVersion, commit, gatewayDigest, vscodeDigests = oldV, oldC, oldGW, oldVS
	}
}
