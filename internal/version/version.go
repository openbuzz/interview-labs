// Package version resolves the interview binary's build identity: the CLI
// version, git commit, and the container image digests it was built against.
package version

import (
	"runtime/debug"
	"strings"
)

// Package-level vars, set via `-ldflags -X` by the release pipeline; all
// stay at their dev defaults for go run / go install / go test.
var (
	buildVersion  = "dev"
	commit        = ""
	gatewayDigest = ""
	vscodeDigests = "" // comma-separated "profile=digest" pairs
)

// Info is the resolved build identity.
type Info struct {
	Version       string
	Commit        string
	GatewayDigest string
	VscodeDigests map[string]string
}

// PinWarning explains tag-fallback resolution on non-release builds.
const PinWarning = "dev build: images resolve by tag (:edge), not by digest — " +
	"released binaries pin exact digests"

// ResolvePins returns the build's Info and, when the binary was not produced
// by the release pipeline, a non-empty warning. A goreleaser --snapshot build
// stamps a real-looking version without collecting digests, so the digests
// themselves are the release signal, not the version string.
func ResolvePins() (Info, string) {
	pinned := gatewayDigest != "" || vscodeDigests != ""
	if buildVersion != "dev" && pinned {
		return Info{
			Version:       buildVersion,
			Commit:        commit,
			GatewayDigest: gatewayDigest,
			VscodeDigests: parseVscodeDigests(vscodeDigests),
		}, ""
	}

	resolved := buildVersion
	if resolved == "dev" {
		resolved = "edge"
		if installed, ok := installedVersion(); ok {
			resolved = strings.TrimPrefix(installed, "v")
		}
	}
	return Info{Version: resolved, Commit: commit,
		VscodeDigests: map[string]string{}}, PinWarning
}

// installedVersion reports the module version stamped by `go install
// pkg@version`; dev trees always report "(devel)". Stamps are v-prefixed
// (v0.1.0) while CI image tags are stripped (0.1.0) — the caller trims.
// A var so tests can stub the build-info read.
var installedVersion = func() (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "", false
	}
	return info.Main.Version, true
}

// parseVscodeDigests parses "profile=digest,…"; malformed pairs are skipped —
// the raw value only ever comes from our own ldflags build step.
func parseVscodeDigests(raw string) map[string]string {
	digests := map[string]string{}
	if raw == "" {
		return digests
	}
	for _, pair := range strings.Split(raw, ",") {
		profile, digest, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		digests[profile] = digest
	}
	return digests
}
