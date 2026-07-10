package cli

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/session"
	sshtest "github.com/openbuzz/interview-labs/internal/ssh"
)

// fakeTFForLaunch installs a fake terraform that emits outputs.json pointing at a
// local in-process ssh server, so the whole pipeline runs without a cloud.
func fakeTFForLaunch(t *testing.T, ip, privPEM, pub string) {
	t.Helper()
	dir := t.TempDir()
	outputs := `{"ip":{"value":"` + ip + `"},` +
		`"ssh_private_key":{"value":` + strconv.Quote(privPEM) + `},` +
		`"ssh_public_key":{"value":` + strconv.Quote(pub) + `}}`
	if err := os.WriteFile(filepath.Join(dir, "outputs.json"),
		[]byte(outputs), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"[ \"$1\" = version ] && echo '{\"terraform_version\":\"1.9.5\"}'\n" +
		"[ \"$1\" = output ] && cat " + filepath.Join(dir, "outputs.json") + "\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+
		string(os.PathListSeparator)+"/usr/bin")
}

func TestLaunchPipelineAgainstLocalSSH(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")

	addr, privPEM, pub := sshtest.StartTestServer(t)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	fakeTFForLaunch(t, host, privPEM, pub)

	oldPort := sshDialPort
	sshDialPort = port
	t.Cleanup(func() { sshDialPort = oldPort })

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if !strings.Contains(out, "Hello world") {
		t.Fatalf("no hello:\n%s", out)
	}
	if !strings.Contains(out, "interview destroy") {
		t.Fatalf("no destroy hint:\n%s", out)
	}

	all, err := session.List()
	if err != nil || len(all) != 1 {
		t.Fatalf("sessions after launch: %d, %v", len(all), err)
	}
	s := all[0]
	if s.Meta.Status != session.StatusReady || s.Meta.IP != host {
		t.Fatalf("meta = %+v", s.Meta)
	}
	if _, err := os.Stat(s.KeyPath()); err != nil {
		t.Fatalf("key not extracted: %v", err)
	}
}

func TestLaunchNonTTYWithoutFlagsExits2(t *testing.T) {
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	old := isTTY
	isTTY = func() bool { return false }
	t.Cleanup(func() { isTTY = old })

	_, code := runCmd(t, "launch")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestLaunchNoTokenFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.Unsetenv("DIGITALOCEAN_TOKEN")

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "interview init") {
		t.Fatalf("missing init hint:\n%s", out)
	}
}
