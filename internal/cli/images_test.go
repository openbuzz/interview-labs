package cli

import "testing"

// Dev builds resolve both images to the edge channel on GHCR.
func TestResolveImagesDevDefaults(t *testing.T) {
	got := resolveImages(&launchFlags{}, "devops")

	if got.Gateway != "ghcr.io/openbuzz/interview-labs-gateway:edge" {
		t.Errorf("gateway = %q", got.Gateway)
	}
	if got.Vscode != "ghcr.io/openbuzz/interview-labs-vscode:edge-devops" {
		t.Errorf("vscode = %q", got.Vscode)
	}
	if got.Warning == "" {
		t.Error("dev build must carry the pin warning")
	}
}

// --tag substitutes bake's local output names, unqualified.
func TestResolveImagesTagLocal(t *testing.T) {
	got := resolveImages(&launchFlags{tag: "local"}, "backend-ai")

	if got.Gateway != "interview-labs-gateway:local" {
		t.Errorf("gateway = %q", got.Gateway)
	}
	if got.Vscode != "interview-labs-vscode:backend-ai-local" {
		t.Errorf("vscode = %q", got.Vscode)
	}
}

// --image / --gateway are verbatim overrides and beat --tag per image.
func TestResolveImagesExplicitOverrides(t *testing.T) {
	f := &launchFlags{tag: "local", image: "example.com/me/vscode:x",
		gateway: "example.com/me/gw:y"}
	got := resolveImages(f, "devops")

	if got.Vscode != "example.com/me/vscode:x" || got.Gateway != "example.com/me/gw:y" {
		t.Errorf("resolved = %+v", got)
	}
}

// Refs reach a remote shell command line verbatim; anything outside the
// docker ref charset is rejected before a launch phase runs.
func TestResolveImagesValidate(t *testing.T) {
	good := resolvedImages{
		Gateway: "ghcr.io/openbuzz/interview-labs-gateway@sha256:abc",
		Vscode:  "interview-labs-vscode:devops-local",
	}
	if err := good.validate(); err != nil {
		t.Errorf("valid refs rejected: %v", err)
	}

	bad := resolvedImages{Gateway: "g'; rm -rf /'", Vscode: "v:1"}
	if bad.validate() == nil {
		t.Error("shell metacharacters must be rejected")
	}
}
