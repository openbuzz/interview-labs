package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	sshtest "github.com/openbuzz/interview-labs/internal/ssh"
)

// fakeTFForLaunch installs a fake terraform that emits an ip output and, on apply,
// drops a keypair into the session ssh dir — mirroring what real terraform now does —
// so the whole pipeline runs against a local in-process ssh server without a cloud.
// Returns the fake's dir so callers can inspect env.txt and the staged tfvars.
func fakeTFForLaunch(t *testing.T, ip, privPEM, pub string) string {
	t.Helper()
	dir := t.TempDir()
	outputs := `{"ip":{"value":"` + ip + `"}}`
	if err := os.WriteFile(filepath.Join(dir, "outputs.json"),
		[]byte(outputs), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "key"), []byte(privPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "key.pub"), []byte(pub), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"[ \"$1\" = version ] && echo '{\"terraform_version\":\"1.9.5\"}'\n" +
		"if [ \"$1\" = apply ]; then\n" +
		"  cp " + filepath.Join(dir, "key") + " ../ssh/key || exit 1\n" +
		"  cp " + filepath.Join(dir, "key.pub") + " ../ssh/key.pub || exit 1\n" +
		"  env >" + filepath.Join(dir, "env.txt") + "\n" +
		"fi\n" +
		"[ \"$1\" = output ] && cat " + filepath.Join(dir, "outputs.json") + "\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+
		string(os.PathListSeparator)+"/usr/bin")
	return dir
}

// swapTTY overrides the isTTY seam for the test's duration.
func swapTTY(t *testing.T, tty bool) {
	t.Helper()
	old := isTTY
	isTTY = func() bool { return tty }
	t.Cleanup(func() { isTTY = old })
}

// assertSessionMeta checks the sole session's provider role, ssh user and image.
func assertSessionMeta(t *testing.T, wantVM, wantUser, wantImage string) *session.Session {
	t.Helper()
	all, err := session.List()
	if err != nil || len(all) != 1 {
		t.Fatalf("sessions: %d, %v", len(all), err)
	}

	s := all[0]
	if s.Meta.Roles["vm"] != wantVM || s.Meta.SSHUser != wantUser ||
		s.Meta.Image != wantImage {
		t.Fatalf("meta = %+v", s.Meta)
	}
	return s
}

// assertEnvDump checks the fake-terraform env capture for a required and a
// forbidden entry.
func assertEnvDump(t *testing.T, dir, want, forbid string) {
	t.Helper()
	envDump, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(envDump), want) {
		t.Fatalf("terraform child env missing %s", want)
	}
	if forbid != "" && strings.Contains(string(envDump), forbid) {
		t.Fatalf("foreign entry %s leaked into terraform env", forbid)
	}
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
	_ = fakeTFForLaunch(t, host, privPEM, pub)

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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	_ = fakeTFForLaunch(t, "127.0.0.1", "", "")
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
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "interview init") {
		t.Fatalf("missing init hint:\n%s", out)
	}
}

func TestLaunchFailsWithoutConfiguredProvider(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	out, code := runCmd(t, "launch")

	if code == 0 {
		t.Fatal("launch succeeded with no providers")
	}
	for _, want := range []string{"No providers configured", "interview init"} {
		if !strings.Contains(out, want) {
			t.Fatalf("gate output missing %q:\n%s", want, out)
		}
	}
}

func TestLaunchPersistsProviderPick(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	swapTTY(t, true)
	oldPick := pickVMProvider
	t.Cleanup(func() { pickVMProvider = oldPick })
	var sawPreselect string
	pickVMProvider = func(configured []provider.Provider,
		preselect string) (provider.Provider, error) {
		sawPreselect = preselect
		return configured[0], nil
	}
	// Make the run stop right after the pick: leave pickRegionSize failing fast.
	oldRS := pickRegionSize
	t.Cleanup(func() { pickRegionSize = oldRS })
	pickRegionSize = func(ctx context.Context, vm provider.VM,
		cfg config.Config) (string, string, error) {
		return "", "", fmt.Errorf("stop here")
	}

	_, _ = runCmd(t, "launch")

	if sawPreselect != "" {
		t.Fatalf("first run preselect = %q, want empty", sawPreselect)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Roles.VM != "digitalocean" {
		t.Fatalf("roles.vm = %q", cfg.Roles.VM)
	}
}

func TestLaunchPersistsPickedRegionAndInstance(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	swapTTY(t, true)
	oldPick, oldRS := pickVMProvider, pickRegionSize
	t.Cleanup(func() { pickVMProvider, pickRegionSize = oldPick, oldRS })
	pickVMProvider = func(configured []provider.Provider,
		_ string) (provider.Provider, error) {
		return configured[0], nil
	}
	var sawPreRegion, sawPreInstance string
	pickRegionSize = func(_ context.Context, vm provider.VM,
		cfg config.Config) (string, string, error) {
		sawPreRegion, sawPreInstance = vm.Defaults(cfg)
		return "fra1", "s-1vcpu-1gb", nil
	}
	// Use the file's existing fake-terraform PATH helper so terraform.Find
	// succeeds; the run may fail later — assert persisted config regardless.
	_ = fakeTFForLaunch(t, "127.0.0.1", "", "")

	_, _ = runCmd(t, "launch")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Providers.DigitalOcean.Region != "fra1" ||
		cfg.Providers.DigitalOcean.Instance != "s-1vcpu-1gb" {
		t.Fatalf("persisted = %+v", cfg.Providers.DigitalOcean)
	}
	if sawPreRegion != "" || sawPreInstance != "" {
		t.Fatalf("first-run preselects = %q/%q, want empty", sawPreRegion, sawPreInstance)
	}
}

func TestLaunchHetznerPipeline(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("HCLOUD_TOKEN", "hz-tok")

	addr, privPEM, pub := sshtest.StartTestServer(t)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	dir := fakeTFForLaunch(t, host, privPEM, pub)

	oldPort := sshDialPort
	sshDialPort = port
	t.Cleanup(func() { sshDialPort = oldPort })

	out, code := runCmd(t, "launch", "--region", "fsn1", "--size", "cx22")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	s := assertSessionMeta(t, "hetzner", "root", "ubuntu-26.04")
	assertEnvDump(t, dir, "HCLOUD_TOKEN=hz-tok", "DIGITALOCEAN_TOKEN=hz-tok")

	tfvars, err := os.ReadFile(
		filepath.Join(s.TerraformDir(), "terraform.tfvars.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tfvars), `"cloud_provider": "hetzner"`) {
		t.Fatalf("tfvars:\n%s", tfvars)
	}
}

func TestLaunchAWSPipeline(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "aws-id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "aws-sec")

	addr, privPEM, pub := sshtest.StartTestServer(t)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	dir := fakeTFForLaunch(t, host, privPEM, pub)

	oldPort := sshDialPort
	sshDialPort = port
	t.Cleanup(func() { sshDialPort = oldPort })

	out, code := runCmd(t, "launch", "--region", "eu-central-1", "--size", "m7i.xlarge")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	all, err := session.List()
	if err != nil || len(all) != 1 {
		t.Fatalf("sessions: %d, %v", len(all), err)
	}
	s := all[0]
	if s.Meta.Roles["vm"] != "aws" || s.Meta.SSHUser != "ubuntu" {
		t.Fatalf("meta = %+v", s.Meta)
	}

	envDump, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"AWS_ACCESS_KEY_ID=aws-id", "AWS_SECRET_ACCESS_KEY=aws-sec",
	} {
		if !strings.Contains(string(envDump), want) {
			t.Fatalf("terraform child env missing %s", want)
		}
	}
}
