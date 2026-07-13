package stack

import (
	"strings"
	"testing"
)

func TestComposeUpCmd(t *testing.T) {
	up := ComposeUpCmd("brave-otter", false)
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
	if got := PushCmd("compose.yaml"); got != want {
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

func TestSetupCmd(t *testing.T) {
	got := SetupCmd("calm-otter", "demo", "hello,world")
	for _, want := range []string{
		"-p interview-calm-otter exec -T",
		"-e INTERVIEW_SESSION_ID=calm-otter",
		"-e INTERVIEW_BUNDLE=demo",
		"-e INTERVIEW_SCENARIOS=hello,world",
		"-e INTERVIEW_LAB_DIR=/opt/interview/lab",
		"vscode bash /opt/interview/lab/setup.sh",
		"-f compose.yaml -f override.yaml",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("SetupCmd missing %q: %q", want, got)
		}
	}
}
