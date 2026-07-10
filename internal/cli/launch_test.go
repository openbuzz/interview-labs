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
	"github.com/openbuzz/interview-labs/internal/digitalocean"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	sshtest "github.com/openbuzz/interview-labs/internal/ssh"
)

// fakeTFForLaunch installs a fake terraform that emits an ip output and, on apply,
// drops a keypair into the session ssh dir — mirroring what real terraform now does —
// so the whole pipeline runs against a local in-process ssh server without a cloud.
func fakeTFForLaunch(t *testing.T, ip, privPEM, pub string) {
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
		"  cp " + filepath.Join(dir, "key") + " ../ssh/key\n" +
		"  cp " + filepath.Join(dir, "key.pub") + " ../ssh/key.pub\n" +
		"fi\n" +
		"[ \"$1\" = output ] && cat " + filepath.Join(dir, "outputs.json") + "\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+
		string(os.PathListSeparator)+"/usr/bin")
}

// swapTTY overrides the isTTY seam for the test's duration.
func swapTTY(t *testing.T, tty bool) {
	t.Helper()
	old := isTTY
	isTTY = func() bool { return tty }
	t.Cleanup(func() { isTTY = old })
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	fakeTFForLaunch(t, "127.0.0.1", "", "")
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

func TestLaunchFailsWithoutConfiguredProvider(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "")

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
	pickRegionSize = func(ctx context.Context, token,
		preRegion, preInstance string) (string, string, error) {
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

func TestSizeLabelShowsHourly(t *testing.T) {
	got := sizeLabel(digitalocean.Size{
		Slug: "s-1vcpu-1gb", VCPUs: 1, Memory: 1024, Disk: 25,
		PriceHourly: 0.00744, PriceMonthly: 5,
	})
	want := "s-1vcpu-1gb  1vcpu 1024MB 25GB  $0.007/hr ($5/mo)"
	if got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}
}

func TestLaunchPersistsPickedRegionAndInstance(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	swapTTY(t, true)
	oldPick, oldRS := pickVMProvider, pickRegionSize
	t.Cleanup(func() { pickVMProvider, pickRegionSize = oldPick, oldRS })
	pickVMProvider = func(configured []provider.Provider,
		_ string) (provider.Provider, error) {
		return configured[0], nil
	}
	var sawPreRegion, sawPreInstance string
	pickRegionSize = func(_ context.Context, _ string,
		preRegion, preInstance string) (string, string, error) {
		sawPreRegion, sawPreInstance = preRegion, preInstance
		return "fra1", "s-1vcpu-1gb", nil
	}
	// Use the file's existing fake-terraform PATH helper so terraform.Find
	// succeeds; the run may fail later — assert persisted config regardless.
	fakeTFForLaunch(t, "127.0.0.1", "", "")

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
