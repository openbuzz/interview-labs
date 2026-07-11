package stack

import (
	"strings"
	"testing"
)

func TestCommands(t *testing.T) {
	if got := ExtractCmd(); got !=
		"mkdir -p /opt/interview/docker && tar -xzf - -C /opt/interview/docker" {
		t.Errorf("ExtractCmd = %q", got)
	}
	if got := BakeCmd("devops-ai"); got !=
		"cd /opt/interview/docker && docker buildx bake gateway devops-ai" {
		t.Errorf("BakeCmd = %q", got)
	}

	up := ComposeUpCmd("brave-otter")
	for _, want := range []string{
		"cd /opt/interview/docker",
		". /dev/stdin",
		"docker compose -p interview-brave-otter up -d --wait",
	} {
		if !strings.Contains(up, want) {
			t.Errorf("ComposeUpCmd missing %q: %q", want, up)
		}
	}
}

func TestEnvBlobSortedAndQuoted(t *testing.T) {
	got := EnvBlob(map[string]string{
		"B_KEY": "plain",
		"A_KEY": "it's quoted",
	})
	want := "A_KEY='it'\\''s quoted'\nB_KEY='plain'\n"
	if got != want {
		t.Errorf("EnvBlob = %q, want %q", got, want)
	}
}
