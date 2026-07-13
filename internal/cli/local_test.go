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
// launch runs: compose up only — the binary never builds images.
func assertLocalDockerCalls(t *testing.T, dir, slug string) {
	t.Helper()
	calls, err := os.ReadFile(filepath.Join(dir, "calls.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "compose -p interview-" + slug + " up -d --wait\n"
	if string(calls) != want {
		t.Fatalf("docker calls:\n%s\nwant:\n%s", calls, want)
	}
}

// assertLocalComposeEnv checks the compose env carries the gateway
// password and image refs, and omits GATEWAY_BIND (loopback default).
func assertLocalComposeEnv(t *testing.T, dir string) {
	t.Helper()
	env, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, wantEnv := range []string{"GATEWAY_PASSWORD=openbuzz",
		"GATEWAY_IMAGE=ghcr.io/openbuzz/interview-labs-gateway:edge",
		"VSCODE_IMAGE=ghcr.io/openbuzz/interview-labs-vscode:edge-devops"} {
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

// blankProviderEnv keeps ambient credentials from configuring providers —
// the local pseudo-provider must be the only configured one in these tests.
func blankProviderEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"DIGITALOCEAN_TOKEN", "HCLOUD_TOKEN",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"OPENROUTER_MANAGEMENT_KEY", "CLOUDFLARE_API_TOKEN"} {
		t.Setenv(k, "")
	}
}

// fakeLocalBins installs a logging stub for each named binary into a
// hermetic PATH (dir alone — no fallback to the real PATH, so a real host
// kind/kubectl can never leak into these tests) and returns the shared
// calls.log path. Mirrors fakeBins in internal/kindx/kindx_test.go.
func fakeLocalBins(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\necho \"${0##*/} $@\" >> " + log + "\nexit 0\n"
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name),
			[]byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return log
}

func TestLocalLaunchDevopsCreatesCluster(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	blankProviderEnv(t)
	swapTTY(t, false)
	log := fakeLocalBins(t, "docker", "kind", "kubectl")

	out, code := runCmd(t, "launch", "--bundle", "devops")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	calls, _ := os.ReadFile(log)
	got := string(calls)
	for _, want := range []string{
		"kind create cluster --name interview-",
		"kubectl --kubeconfig", "apply -f",
		"kind get kubeconfig", "--internal",
		"docker compose",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("calls missing %q:\n%s", want, got)
		}
	}

	all, _ := session.List()
	s := all[0]
	if !s.Meta.Kind {
		t.Fatalf("meta.Kind = false")
	}
	if _, err := os.Stat(filepath.Join(s.StackDir(), "override.yaml")); err != nil {
		t.Fatalf("override not staged: %v", err)
	}
}

func TestLocalDestroyDeletesCluster(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	blankProviderEnv(t)
	swapTTY(t, false)
	log := fakeLocalBins(t, "docker", "kind", "kubectl")

	if _, code := runCmd(t, "launch", "--bundle", "devops"); code != 0 {
		t.Fatal("launch failed")
	}
	all, _ := session.List()
	slug := all[0].Meta.Slug

	if _, code := runCmd(t, "destroy", slug, "--yes"); code != 0 {
		t.Fatal("destroy failed")
	}

	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "kind delete cluster --name interview-"+slug) {
		t.Fatalf("no cluster delete in:\n%s", string(calls))
	}
}

func TestLocalKindBundleNeedsHostBins(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	blankProviderEnv(t)
	swapTTY(t, false)
	_ = fakeLocalBins(t, "docker") // docker only — no kind/kubectl

	out, code := runCmd(t, "launch", "--bundle", "devops")
	if code == 0 || !strings.Contains(out, "kind") {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if all, _ := session.List(); len(all) != 0 {
		t.Fatal("failed gate still created a session")
	}
}
