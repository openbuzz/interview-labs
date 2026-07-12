package stack

import (
	"strings"
	"testing"
)

func TestComposeUpCmd(t *testing.T) {
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

func TestPushCmd(t *testing.T) {
	want := "mkdir -p /opt/interview/docker && " +
		"cat > /opt/interview/docker/compose.yaml"
	if got := PushCmd(); got != want {
		t.Errorf("PushCmd = %q", got)
	}
}

func TestPullCmd(t *testing.T) {
	want := "cd /opt/interview/docker && " +
		"GATEWAY_IMAGE='g:1' VSCODE_IMAGE='v:1' docker compose pull"
	if got := PullCmd("g:1", "v:1"); got != want {
		t.Errorf("PullCmd = %q", got)
	}
}
