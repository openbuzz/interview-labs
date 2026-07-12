package cli

import (
	"fmt"
	"regexp"

	"github.com/openbuzz/interview-labs/internal/version"
)

// imageRegistry is the first-party image registry (spec §4).
const imageRegistry = "ghcr.io/openbuzz/"

// resolvedImages is the two launchable refs a session pulls, plus the
// non-empty pin warning on dev builds.
type resolvedImages struct {
	Gateway, Vscode string
	Warning         string
}

// resolveImages applies flag overrides over the build pins. Precedence per
// image: explicit ref flag > --tag substitution > digest pin > channel tag.
func resolveImages(f *launchFlags, profile string) resolvedImages {
	pins, warning := version.ResolvePins()

	gateway := f.gateway
	if gateway == "" {
		gateway = gatewayRef(pins, f.tag)
	}
	vscode := f.image
	if vscode == "" {
		vscode = vscodeRef(pins, profile, f.tag)
	}
	return resolvedImages{Gateway: gateway, Vscode: vscode, Warning: warning}
}

// gatewayRef resolves the gateway image: --tag names a bake output tag
// (unqualified, e.g. :local); digest pins beat the channel tag.
func gatewayRef(pins version.Info, tag string) string {
	if tag != "" {
		return "interview-labs-gateway:" + tag
	}
	if pins.GatewayDigest != "" {
		return imageRegistry + "interview-labs-gateway@" + pins.GatewayDigest
	}
	return imageRegistry + "interview-labs-gateway:" + pins.Version
}

// vscodeRef resolves the profile's vscode image the same way; tags carry the
// profile suffix (bake local output and CI publish share the shape).
func vscodeRef(pins version.Info, profile, tag string) string {
	if tag != "" {
		return "interview-labs-vscode:" + profile + "-" + tag
	}
	if digest, ok := pins.VscodeDigests[profile]; ok && digest != "" {
		return imageRegistry + "interview-labs-vscode@" + digest
	}
	return imageRegistry + "interview-labs-vscode:" + pins.Version + "-" + profile
}

// imageRefPattern approximates a docker reference (name[:tag][@digest]) and
// excludes shell metacharacters — resolved refs land verbatim on the remote
// pull command line.
var imageRefPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]*$`)

// validate rejects refs that could break or escape the remote shell command.
func (r resolvedImages) validate() error {
	for _, ref := range []string{r.Gateway, r.Vscode} {
		if !imageRefPattern.MatchString(ref) {
			return fmt.Errorf("invalid image ref %q", ref)
		}
	}
	return nil
}
