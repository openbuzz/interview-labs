package stack

import (
	"strings"
	"testing"
)

func TestOverrideFull(t *testing.T) {
	out := Override(true, true, true)
	for _, want := range []string{
		"./payload/workspace:/home/user/scenarios:rw",
		"./payload/lab:/opt/interview/lab:ro",
		"./payload/kubeconfig:/home/user/.kube/config:ro",
		"networks: [default, kind]",
		"external: true",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("override missing %q:\n%s", want, out)
		}
	}
}

func TestOverrideWorkspaceOnly(t *testing.T) {
	out := Override(true, false, false)
	if strings.Contains(out, "kind") || strings.Contains(out, "lab") {
		t.Fatalf("kindless override leaks mounts:\n%s", out)
	}
	if !strings.Contains(out, "./payload/workspace:/home/user/scenarios:rw") {
		t.Fatalf("workspace mount missing:\n%s", out)
	}
}

func TestComposeUpCmdVariants(t *testing.T) {
	plain := ComposeUpCmd("slug1", false)
	if strings.Contains(plain, "override") {
		t.Fatalf("bare up references override: %q", plain)
	}
	over := ComposeUpCmd("slug1", true)
	if !strings.Contains(over, "-f compose.yaml -f override.yaml") {
		t.Fatalf("override up = %q", over)
	}
}

func TestPushCmdNames(t *testing.T) {
	if got := PushCmd("compose.yaml"); !strings.Contains(got, "docker/compose.yaml") {
		t.Fatalf("PushCmd = %q", got)
	}
	if got := PushCmd("override.yaml"); !strings.Contains(got, "docker/override.yaml") {
		t.Fatalf("PushCmd = %q", got)
	}
}
