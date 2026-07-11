package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/session"
)

// fakeDocker installs a docker stub logging each invocation's argv (one
// line per call) plus an env snapshot, always exiting 0.
func fakeDocker(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"echo \"$@\" >>" + filepath.Join(dir, "calls.txt") + "\n" +
		"env >>" + filepath.Join(dir, "env.txt") + "\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+
		string(os.PathListSeparator)+"/usr/bin")
	return dir
}

// blankCloudEnv guarantees no cloud provider is configured, so the local
// pseudo-provider is the sole vm-role candidate.
func blankCloudEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
}

func TestLocalLaunchRunsStackOnHostDocker(t *testing.T) {
	blankCloudEnv(t)
	dir := fakeDocker(t)

	out, code := runCmd(t, "launch")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	all, err := session.List()
	if err != nil || len(all) != 1 {
		t.Fatalf("sessions: %d, %v", len(all), err)
	}
	s := all[0]
	assertLocalSessionMeta(t, s)
	assertLocalDockerCalls(t, dir, s.Meta.Slug)
	assertLocalComposeEnv(t, dir)

	if _, err := os.Stat(filepath.Join(s.StackDir(), "compose.yaml")); err != nil {
		t.Fatalf("stack not staged: %v", err)
	}
	for _, wantOut := range []string{"http://localhost:8080", "openbuzz"} {
		if !strings.Contains(out, wantOut) {
			t.Fatalf("handover missing %q:\n%s", wantOut, out)
		}
	}
}

// assertLocalSessionMeta checks the local session's role/profile/status,
// its handover URL/password, and the absence of ssh fields.
func assertLocalSessionMeta(t *testing.T, s *session.Session) {
	t.Helper()
	if s.Meta.Roles["vm"] != "local" || s.Meta.Profile != "devops" ||
		s.Meta.Status != session.StatusReady {
		t.Fatalf("meta = %+v", s.Meta)
	}
	if s.Meta.URL != "http://localhost:8080" || s.Meta.GatewayPassword != "openbuzz" {
		t.Fatalf("handover meta = %+v", s.Meta)
	}
	if s.Meta.SSHUser != "" || s.Meta.IP != "" {
		t.Fatalf("local session must have no ssh_user/ip: %+v", s.Meta)
	}
}

// assertLocalDockerCalls checks the exact docker argv sequence a local
// launch runs: build the stack, then compose up.
func assertLocalDockerCalls(t *testing.T, dir, slug string) {
	t.Helper()
	calls, err := os.ReadFile(filepath.Join(dir, "calls.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "buildx bake gateway devops\n" +
		"compose -p interview-" + slug + " up -d --wait\n"
	if string(calls) != want {
		t.Fatalf("docker calls:\n%s\nwant:\n%s", calls, want)
	}
}

// assertLocalComposeEnv checks the compose env carries the gateway
// password and profile, and omits GATEWAY_BIND (loopback default).
func assertLocalComposeEnv(t *testing.T, dir string) {
	t.Helper()
	env, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, wantEnv := range []string{"GATEWAY_PASSWORD=openbuzz",
		"VSCODE_PROFILE=devops"} {
		if !strings.Contains(string(env), wantEnv) {
			t.Fatalf("compose env missing %s", wantEnv)
		}
	}
	if strings.Contains(string(env), "GATEWAY_BIND=") {
		t.Fatal("local must not set GATEWAY_BIND (loopback default)")
	}
}

func TestLocalLaunchRefusesSecondSession(t *testing.T) {
	blankCloudEnv(t)
	_ = fakeDocker(t)

	if out, code := runCmd(t, "launch"); code != 0 {
		t.Fatalf("first launch: %d\n%s", code, out)
	}
	out, code := runCmd(t, "launch")

	if code != 1 {
		t.Fatalf("second launch exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "one local stack at a time") {
		t.Fatalf("gate message missing:\n%s", out)
	}
}

func TestLocalLaunchAIProfileMintsAndInjectsKey(t *testing.T) {
	blankCloudEnv(t)
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	dir := fakeDocker(t)
	_, mintCalls := mintServer(t, "hash-1")

	out, code := runCmd(t, "launch", "--profile", "devops-ai")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	if mintCalls.Load() != 1 {
		t.Fatalf("mint calls = %d", mintCalls.Load())
	}
	env, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "OPENROUTER_API_KEY=sk-or-child") {
		t.Fatal("compose env missing the minted key")
	}
	all, _ := session.List()
	raw, _ := os.ReadFile(all[0].MetadataPath())
	if strings.Contains(string(raw), "sk-or-child") {
		t.Fatalf("key value leaked into metadata:\n%s", raw)
	}
}

func TestLocalDestroyComposeDownAndArchive(t *testing.T) {
	blankCloudEnv(t)
	dir := fakeDocker(t)
	if out, code := runCmd(t, "launch"); code != 0 {
		t.Fatalf("launch: %d\n%s", code, out)
	}
	all, _ := session.List()
	slug := all[0].Meta.Slug

	out, code := runCmd(t, "destroy", slug, "--yes")
	if code != 0 {
		t.Fatalf("destroy exit = %d\n%s", code, out)
	}

	calls, err := os.ReadFile(filepath.Join(dir, "calls.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "compose -p interview-"+slug+" down -v\n") {
		t.Fatalf("compose down missing:\n%s", calls)
	}
	if left, _ := session.List(); len(left) != 0 {
		t.Fatalf("session not archived: %d left", len(left))
	}
	if !strings.Contains(out, "destroyed") {
		t.Fatalf("no destroyed row:\n%s", out)
	}
}

func TestLocalSSHExecsIntoVSCodeContainer(t *testing.T) {
	blankCloudEnv(t)
	_ = fakeDocker(t)
	if out, code := runCmd(t, "launch"); code != 0 {
		t.Fatalf("launch: %d\n%s", code, out)
	}
	all, _ := session.List()
	slug := all[0].Meta.Slug

	oldExec := execProgram
	t.Cleanup(func() { execProgram = oldExec })
	var gotArgv []string
	execProgram = func(argv []string) error {
		gotArgv = argv
		return nil
	}

	if out, code := runCmd(t, "ssh", slug); code != 0 {
		t.Fatalf("ssh exit = %d\n%s", code, out)
	}
	want := "docker exec -it interview-" + slug + "-vscode bash"
	if strings.Join(gotArgv, " ") != want {
		t.Fatalf("argv = %v, want %q", gotArgv, want)
	}
}
