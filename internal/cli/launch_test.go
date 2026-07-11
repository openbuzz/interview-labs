package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/openrouter"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	sshtest "github.com/openbuzz/interview-labs/internal/ssh"
	"github.com/openbuzz/interview-labs/internal/ui"
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

// assertRemoteCommands checks the exact ssh exec sequence a cloud launch
// runs, in order: cloud-init wait, push stack, build stack, compose up.
func assertRemoteCommands(t *testing.T, rec *sshtest.ExecRecorder, slug string) {
	t.Helper()
	want := []string{
		"cloud-init status --wait",
		"mkdir -p /opt/interview/docker && tar -xzf - -C /opt/interview/docker",
		"cd /opt/interview/docker && docker buildx bake gateway devops",
		"cd /opt/interview/docker && set -a && . /dev/stdin && set +a && " +
			"docker compose -p interview-" + slug + " up -d --wait",
	}
	got := rec.Commands()
	if len(got) != len(want) {
		t.Fatalf("remote commands = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("command %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// assertContainsAll checks out contains every wanted substring.
func assertContainsAll(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
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
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")

	addr, privPEM, pub, rec := sshtest.StartRecordingTestServer(t)
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

	all, err := session.List()
	if err != nil || len(all) != 1 {
		t.Fatalf("sessions after launch: %d, %v", len(all), err)
	}
	s := all[0]
	assertRemoteCommands(t, rec, s.Meta.Slug)

	if s.Meta.Status != session.StatusReady || s.Meta.IP != host {
		t.Fatalf("meta = %+v", s.Meta)
	}
	if s.Meta.URL != "http://"+host {
		t.Fatalf("url = %q", s.Meta.URL)
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(s.Meta.GatewayPassword) {
		t.Fatalf("password = %q", s.Meta.GatewayPassword)
	}
	assertContainsAll(t, out, "http://"+host, s.Meta.GatewayPassword, "interview destroy")
	if _, err := os.Stat(s.KeyPath()); err != nil {
		t.Fatalf("key not extracted: %v", err)
	}
}

func TestLaunchNonTTYWithoutFlagsExits2(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
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
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("PATH", t.TempDir()) // no docker ⇒ local stays unconfigured

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
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("PATH", t.TempDir()) // no docker ⇒ local stays unconfigured

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
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
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
	pickRegionSize = func(_ context.Context, _ io.Writer, _ provider.VM,
		_ config.Config) (provider.Option, provider.SizeInfo, error) {
		return provider.Option{}, provider.SizeInfo{}, fmt.Errorf("stop here")
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
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	swapTTY(t, true)
	oldPick, oldRS := pickVMProvider, pickRegionSize
	t.Cleanup(func() { pickVMProvider, pickRegionSize = oldPick, oldRS })
	oldProfile := pickProfile
	t.Cleanup(func() { pickProfile = oldProfile })
	pickVMProvider = func(configured []provider.Provider,
		_ string) (provider.Provider, error) {
		return configured[0], nil
	}
	pickProfile = func(_ string) (string, error) { return "devops", nil }
	var sawPreRegion, sawPreInstance string
	pickRegionSize = func(_ context.Context, _ io.Writer, vm provider.VM,
		cfg config.Config) (provider.Option, provider.SizeInfo, error) {
		sawPreRegion, sawPreInstance = vm.Defaults(cfg)
		return provider.Option{Slug: "fra1", Label: "fra1  Frankfurt 1"},
			provider.SizeInfo{Slug: "s-1vcpu-1gb"}, nil
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
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")

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
	if !strings.Contains(string(tfvars), `"user_data": "#cloud-config`) {
		t.Fatalf("tfvars missing user_data:\n%s", tfvars)
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
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")

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

func TestLaunchQuietRendersPhaseRows(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	swapTTY(t, true)
	oldUI := ui.Interactive
	ui.Interactive = func() bool { return false } // plain step lines, no ANSI redraw
	t.Cleanup(func() { ui.Interactive = oldUI })
	// swapTTY(true) also routes selectVMProvider and ensureProfile through
	// the real huh pickers; stub both like the sibling TTY tests do so the
	// sandbox (no /dev/tty) doesn't block.
	oldPick := pickVMProvider
	t.Cleanup(func() { pickVMProvider = oldPick })
	pickVMProvider = func(configured []provider.Provider,
		_ string) (provider.Provider, error) {
		return configured[0], nil
	}
	oldProfile := pickProfile
	t.Cleanup(func() { pickProfile = oldProfile })
	pickProfile = func(_ string) (string, error) { return "devops", nil }

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

	// --yes: this test is about phase rows, not the confirm gate; skip it
	// like the real huh picker above, same reason.
	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb", "--yes")

	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	for _, row := range []string{"stage", "terraform init", "terraform apply",
		"wait-ssh", "cloud-init", "push stack", "build stack", "compose up"} {
		if !strings.Contains(out, ui.GlyphOK+" "+row) {
			t.Fatalf("missing quiet row %q:\n%s", row, out)
		}
	}
}

func TestLaunchVerboseStreamsTerraform(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	swapTTY(t, true)
	// swapTTY(true) also routes selectVMProvider and ensureProfile through
	// the real huh pickers; stub both like the sibling TTY tests do so the
	// sandbox (no /dev/tty) doesn't block.
	oldPick := pickVMProvider
	t.Cleanup(func() { pickVMProvider = oldPick })
	pickVMProvider = func(configured []provider.Provider,
		_ string) (provider.Provider, error) {
		return configured[0], nil
	}
	oldProfile := pickProfile
	t.Cleanup(func() { pickProfile = oldProfile })
	pickProfile = func(_ string) (string, error) { return "devops", nil }

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

	// --yes: this test is about verbose streaming, not the confirm gate;
	// skip it like the real huh picker above, same reason.
	out, code := runCmd(t, "--verbose", "launch",
		"--region", "fra1", "--size", "s-1vcpu-1gb", "--yes")

	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if strings.Contains(out, ui.GlyphOK+" terraform apply") {
		t.Fatalf("verbose mode rendered quiet rows:\n%s", out)
	}
}

// loopVM is a minimal provider.VM fake for the region/size loop tests; only
// Regions, Sizes and Defaults return real data.
type loopVM struct {
	regions []provider.Option
	sizes   map[string][]provider.SizeInfo
}

func (v loopVM) Name() string  { return "loopvm" }
func (v loopVM) Label() string { return "LoopVM" }
func (v loopVM) Roles() []provider.Role {
	return []provider.Role{provider.RoleVM}
}
func (v loopVM) Configured(config.Config) bool                      { return true }
func (v loopVM) Configure(context.Context, *config.Config) error    { return nil }
func (v loopVM) Image() string                                      { return "img" }
func (v loopVM) SSHUser() string                                    { return "root" }
func (v loopVM) EnvCreds(config.Config) map[string]string           { return nil }
func (v loopVM) ValidateCreds(context.Context, config.Config) error { return nil }
func (v loopVM) Regions(context.Context, config.Config) ([]provider.Option, error) {
	return v.regions, nil
}
func (v loopVM) Sizes(_ context.Context, _ config.Config,
	region string) ([]provider.SizeInfo, error) {
	return v.sizes[region], nil
}
func (v loopVM) Defaults(config.Config) (string, string)    { return "", "" }
func (v loopVM) SetDefaults(*config.Config, string, string) {}

func TestPickRegionSizeEscAtSizeReturnsToRegion(t *testing.T) {
	oldR, oldS := pickRegionForm, pickSizeForm
	t.Cleanup(func() { pickRegionForm, pickSizeForm = oldR, oldS })

	vm := loopVM{
		regions: []provider.Option{{Slug: "fra1", Label: "fra1  Frankfurt 1"},
			{Slug: "nyc1", Label: "nyc1  New York 1"}},
		sizes: map[string][]provider.SizeInfo{
			"fra1": {{Slug: "b1", Category: "Basic", VCPUs: 2, MemGB: 4, DiskGB: 80,
				Hourly: 0.036, Currency: "$"}},
			"nyc1": {{Slug: "b2", Category: "Basic", VCPUs: 4, MemGB: 8, DiskGB: 160,
				Hourly: 0.071, Currency: "$"}},
		},
	}

	regionPicks := []string{"fra1", "nyc1"}
	pickRegionForm = func(_ []huh.Option[string], _ string) (string, error) {
		p := regionPicks[0]
		regionPicks = regionPicks[1:]
		return p, nil
	}
	sizeCalls := 0
	pickSizeForm = func(opts []huh.Option[string], _ string) (string, error) {
		sizeCalls++
		if sizeCalls == 1 {
			return "", huh.ErrUserAborted // ESC at size -> back to region
		}
		return "b2", nil
	}

	var out bytes.Buffer
	region, si, err := pickRegionSize(context.Background(), &out, vm, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if region.Slug != "nyc1" || si.Slug != "b2" {
		t.Fatalf("picked %s/%s, want nyc1/b2", region.Slug, si.Slug)
	}
	if len(regionPicks) != 0 || sizeCalls != 2 {
		t.Fatalf("region picker not re-shown after ESC (size calls %d)", sizeCalls)
	}
}

func TestPickRegionSizeEmptySizesRepicksRegion(t *testing.T) {
	oldR, oldS := pickRegionForm, pickSizeForm
	t.Cleanup(func() { pickRegionForm, pickSizeForm = oldR, oldS })

	vm := loopVM{
		regions: []provider.Option{{Slug: "empty1", Label: "empty1"},
			{Slug: "fra1", Label: "fra1  Frankfurt 1"}},
		sizes: map[string][]provider.SizeInfo{
			"fra1": {{Slug: "b1", Category: "Basic", VCPUs: 2, MemGB: 4, DiskGB: 80,
				Hourly: 0.036, Currency: "$"}},
		},
	}

	regionPicks := []string{"empty1", "fra1"}
	pickRegionForm = func(_ []huh.Option[string], _ string) (string, error) {
		p := regionPicks[0]
		regionPicks = regionPicks[1:]
		return p, nil
	}
	pickSizeForm = func(_ []huh.Option[string], _ string) (string, error) {
		return "b1", nil
	}

	var out bytes.Buffer
	region, si, err := pickRegionSize(context.Background(), &out, vm, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if region.Slug != "fra1" || si.Slug != "b1" {
		t.Fatalf("picked %s/%s, want fra1/b1", region.Slug, si.Slug)
	}
	if !strings.Contains(out.String(), "no matching sizes") {
		t.Fatalf("empty-region warning missing from output: %q", out.String())
	}
}

func TestPickRegionSizeEscAtRegionAborts(t *testing.T) {
	oldR := pickRegionForm
	t.Cleanup(func() { pickRegionForm = oldR })
	pickRegionForm = func(_ []huh.Option[string], _ string) (string, error) {
		return "", huh.ErrUserAborted
	}

	vm := loopVM{regions: []provider.Option{{Slug: "fra1", Label: "fra1"}}}
	var out bytes.Buffer
	_, _, err := pickRegionSize(context.Background(), &out, vm, config.Config{})
	if !errors.Is(err, huh.ErrUserAborted) {
		t.Fatalf("err = %v, want huh.ErrUserAborted", err)
	}
}

func TestPickRegionSizeSortsCheapestFirst(t *testing.T) {
	oldR, oldS := pickRegionForm, pickSizeForm
	t.Cleanup(func() { pickRegionForm, pickSizeForm = oldR, oldS })

	vm := loopVM{
		regions: []provider.Option{{Slug: "fra1", Label: "fra1"}},
		sizes: map[string][]provider.SizeInfo{
			"fra1": {
				{Slug: "pricey", Category: "Basic", Hourly: 0.10, Currency: "$"},
				{Slug: "cheap", Category: "Basic", Hourly: 0.02, Currency: "$"},
				{Slug: "tie-b", Category: "Basic", Hourly: 0.02, Currency: "$"},
			},
		},
	}
	pickRegionForm = func(_ []huh.Option[string], _ string) (string, error) {
		return "fra1", nil
	}
	var sawFirst string
	pickSizeForm = func(opts []huh.Option[string], _ string) (string, error) {
		sawFirst = opts[0].Value
		return opts[0].Value, nil
	}

	var out bytes.Buffer
	if _, _, err := pickRegionSize(context.Background(), &out, vm,
		config.Config{}); err != nil {
		t.Fatal(err)
	}
	if sawFirst != "cheap" {
		t.Fatalf("first option = %q, want cheap (price sort, slug tie-break)", sawFirst)
	}
}

func TestLaunchConfirmNoCancelsCleanly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	swapTTY(t, true)
	_ = fakeTFForLaunch(t, "127.0.0.1", "", "")

	oldPick, oldRS, oldC := pickVMProvider, pickRegionSize, confirmLaunch
	t.Cleanup(func() {
		pickVMProvider, pickRegionSize, confirmLaunch = oldPick, oldRS, oldC
	})
	oldProfile := pickProfile
	t.Cleanup(func() { pickProfile = oldProfile })
	pickVMProvider = func(configured []provider.Provider,
		_ string) (provider.Provider, error) {
		return configured[0], nil
	}
	pickProfile = func(_ string) (string, error) { return "devops", nil }
	pickRegionSize = func(_ context.Context, _ io.Writer, _ provider.VM,
		_ config.Config) (provider.Option, provider.SizeInfo, error) {
		return provider.Option{Slug: "fra1", Label: "fra1  Frankfurt 1"},
			provider.SizeInfo{Slug: "s-2vcpu-4gb", Category: "Basic", VCPUs: 2,
				MemGB: 4, DiskGB: 80, Hourly: 0.036, Currency: "$"}, nil
	}
	confirmLaunch = func() (bool, error) { return false, nil }

	out, code := runCmd(t, "launch")
	if code != 0 {
		t.Fatalf("declining the confirm must exit clean, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "Launch summary") ||
		!strings.Contains(out, "Basic — 2 vCPU, 4 GB memory, 80 GB disk") ||
		!strings.Contains(out, `~$0.04/h, billed until "interview destroy"`) {
		t.Fatalf("summary box missing or wrong:\n%s", out)
	}
	if !strings.Contains(out, "launch cancelled — nothing provisioned") {
		t.Fatalf("cancel notice missing:\n%s", out)
	}
}

func TestLaunchYesSkipsConfirmButPrintsSummary(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	swapTTY(t, true)
	_ = fakeTFForLaunch(t, "127.0.0.1", "", "")

	oldPick, oldRS, oldC := pickVMProvider, pickRegionSize, confirmLaunch
	t.Cleanup(func() {
		pickVMProvider, pickRegionSize, confirmLaunch = oldPick, oldRS, oldC
	})
	oldProfile := pickProfile
	t.Cleanup(func() { pickProfile = oldProfile })
	pickVMProvider = func(configured []provider.Provider,
		_ string) (provider.Provider, error) {
		return configured[0], nil
	}
	pickProfile = func(_ string) (string, error) { return "devops", nil }
	pickRegionSize = func(_ context.Context, _ io.Writer, _ provider.VM,
		_ config.Config) (provider.Option, provider.SizeInfo, error) {
		return provider.Option{Slug: "fra1", Label: "fra1  Frankfurt 1"},
			provider.SizeInfo{Slug: "s-2vcpu-4gb", Category: "Basic", VCPUs: 2,
				MemGB: 4, DiskGB: 80, Hourly: 0.036, Currency: "$"}, nil
	}
	confirmLaunch = func() (bool, error) {
		t.Fatal("--yes must not prompt")
		return false, nil
	}

	// The run proceeds past the gate and may fail later in terraform; only
	// the gate behavior is asserted.
	out, _ := runCmd(t, "launch", "--yes")
	if !strings.Contains(out, "Launch summary") {
		t.Fatalf("summary box must still print with --yes:\n%s", out)
	}
}

func TestLaunchNonTTYSkipsSummaryAndConfirm(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	swapTTY(t, false)
	_ = fakeTFForLaunch(t, "127.0.0.1", "", "")

	oldC := confirmLaunch
	t.Cleanup(func() { confirmLaunch = oldC })
	confirmLaunch = func() (bool, error) {
		t.Fatal("non-TTY must not prompt")
		return false, nil
	}

	out, _ := runCmd(t, "launch", "--region", "fra1", "--size", "s-2vcpu-4gb")
	if strings.Contains(out, "Launch summary") {
		t.Fatalf("non-TTY printed the summary box:\n%s", out)
	}
}

func TestLaunchSummaryFlagsPathFallsBackToSlugs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	swapTTY(t, true)
	_ = fakeTFForLaunch(t, "127.0.0.1", "", "")

	oldPick, oldC := pickVMProvider, confirmLaunch
	t.Cleanup(func() { pickVMProvider, confirmLaunch = oldPick, oldC })
	oldProfile := pickProfile
	t.Cleanup(func() { pickProfile = oldProfile })
	pickVMProvider = func(configured []provider.Provider,
		_ string) (provider.Provider, error) {
		return configured[0], nil
	}
	pickProfile = func(_ string) (string, error) { return "devops", nil }
	confirmLaunch = func() (bool, error) { return false, nil }

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-2vcpu-4gb")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	// Flags path has no SizeInfo: region and size rows show the raw slugs,
	// and no price row is rendered.
	if !strings.Contains(out, "Launch summary") || !strings.Contains(out, "fra1") ||
		!strings.Contains(out, "s-2vcpu-4gb") {
		t.Fatalf("flags-path summary wrong:\n%s", out)
	}
	if strings.Contains(out, "/h,") {
		t.Fatalf("flags-path summary must omit the price row:\n%s", out)
	}
}

// mintServer fakes OpenRouter's POST /keys; counts requests.
func mintServer(t *testing.T, hash string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/keys" {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			}
			calls.Add(1)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"key":"sk-or-child","data":{"hash":"` + hash + `"}}`))
		}))
	t.Cleanup(srv.Close)
	old := openrouter.BaseURL
	openrouter.BaseURL = srv.URL
	t.Cleanup(func() { openrouter.BaseURL = old })
	return srv, &calls
}

// writeCFConfig stores cloudflare zone config (zone_id is config-only).
func writeCFConfig(t *testing.T) {
	t.Helper()
	cfg := config.Config{}
	cfg.Providers.Cloudflare.ZoneID = "z1"
	cfg.Providers.Cloudflare.Domain = "example.test"
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchWithAIAndDNS(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cf-tok")
	writeCFConfig(t)
	_, mintCalls := mintServer(t, "hash-1")

	addr, privPEM, pub := sshtest.StartTestServer(t)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	dir := fakeTFForLaunch(t, host, privPEM, pub)
	// The fake's outputs gain the fqdn the real root module now emits.
	outputs := `{"ip":{"value":"` + host + `"},` +
		`"fqdn":{"value":"session.example.test"}}`
	if err := os.WriteFile(filepath.Join(dir, "outputs.json"),
		[]byte(outputs), 0o644); err != nil {
		t.Fatal(err)
	}
	oldPort := sshDialPort
	sshDialPort = port
	t.Cleanup(func() { sshDialPort = oldPort })

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb",
		"--profile", "devops-ai")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	if mintCalls.Load() != 1 {
		t.Fatalf("mint calls = %d, want 1", mintCalls.Load())
	}
	all, err := session.List()
	if err != nil || len(all) != 1 {
		t.Fatalf("sessions: %d, %v", len(all), err)
	}
	s := all[0]
	assertAIAndDNSMeta(t, s, "hash-1", 10, "session.example.test")

	assertEnvDump(t, dir, "CLOUDFLARE_API_TOKEN=cf-tok", "")
	assertTfvarsContains(t, s, `"dns_enabled": true`, `"cloudflare_zone_id": "z1"`,
		`"dns_base_domain": "example.test"`)
}

// assertAIAndDNSMeta checks the roles a launch recorded for the AI and
// access providers, plus the minted key's persisted hash/cap and DNS name.
func assertAIAndDNSMeta(t *testing.T, s *session.Session, wantHash string,
	wantCapUSD float64, wantFQDN string) {
	t.Helper()
	if s.Meta.Roles["ai"] != "openrouter" || s.Meta.Roles["access"] != "cloudflare" {
		t.Fatalf("roles = %v", s.Meta.Roles)
	}
	if s.Meta.AIKeyHash != wantHash || s.Meta.AICapUSD != wantCapUSD ||
		s.Meta.FQDN != wantFQDN {
		t.Fatalf("meta = %+v", s.Meta)
	}
}

// assertTfvarsContains checks the session's staged tfvars file contains
// every wanted substring.
func assertTfvarsContains(t *testing.T, s *session.Session, wants ...string) {
	t.Helper()
	tfvars, err := os.ReadFile(filepath.Join(s.TerraformDir(), "terraform.tfvars.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range wants {
		if !strings.Contains(string(tfvars), want) {
			t.Fatalf("tfvars missing %s:\n%s", want, tfvars)
		}
	}
}

func TestLaunchOptOutFlagsSkipAIAndDNS(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cf-tok")
	writeCFConfig(t)
	_, mintCalls := mintServer(t, "hash-1")

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

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb",
		"--no-ai", "--no-dns")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	if mintCalls.Load() != 0 {
		t.Fatalf("mint called %d times despite --no-ai", mintCalls.Load())
	}
	all, _ := session.List()
	s := all[0]
	if _, ok := s.Meta.Roles["ai"]; ok {
		t.Fatalf("roles = %v", s.Meta.Roles)
	}
	if _, ok := s.Meta.Roles["access"]; ok {
		t.Fatalf("roles = %v", s.Meta.Roles)
	}
	tfvars, _ := os.ReadFile(filepath.Join(s.TerraformDir(), "terraform.tfvars.json"))
	if strings.Contains(string(tfvars), "dns_enabled") {
		t.Fatalf("tfvars must omit dns vars with --no-dns:\n%s", tfvars)
	}
}

func TestLaunchMintFailureFailsSession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"message":"quota exceeded"}}`))
		}))
	t.Cleanup(srv.Close)
	oldBase := openrouter.BaseURL
	openrouter.BaseURL = srv.URL
	t.Cleanup(func() { openrouter.BaseURL = oldBase })

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

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb",
		"--profile", "devops-ai")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}

	all, _ := session.List()
	if len(all) != 1 {
		t.Fatalf("sessions = %d", len(all))
	}
	s := all[0]
	if s.Meta.Status != session.StatusFailed || s.Meta.AIKeyHash != "" {
		t.Fatalf("meta = %+v", s.Meta)
	}
	if !strings.Contains(out, "interview destroy") {
		t.Fatalf("missing destroy hint:\n%s", out)
	}
}

func TestLaunchProfileFlagRejected(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb",
		"--profile", "bogus")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "backend, devops, backend-ai, devops-ai") {
		t.Fatalf("error must list profiles:\n%s", out)
	}
}

func TestLaunchNonTTYDefaultsDevopsAndSkipsMint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	_, mintCalls := mintServer(t, "hash-1")

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

	if mintCalls.Load() != 0 {
		t.Fatalf("devops profile must not mint (calls = %d)", mintCalls.Load())
	}
	all, _ := session.List()
	if len(all) != 1 || all[0].Meta.Profile != "devops" {
		t.Fatalf("profile = %+v", all[0].Meta)
	}
	if _, ok := all[0].Meta.Roles["ai"]; ok {
		t.Fatalf("non-ai profile must not record an ai role: %v", all[0].Meta.Roles)
	}
}

func TestLaunchAIProfileMints(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	_, mintCalls := mintServer(t, "hash-1")

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

	out, code := runCmd(t, "launch", "--region", "fra1", "--size", "s-1vcpu-1gb",
		"--profile", "devops-ai")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	if mintCalls.Load() != 1 {
		t.Fatalf("mint calls = %d, want 1", mintCalls.Load())
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "devops-ai" {
		t.Fatalf("config profile = %q, want devops-ai (remembered)", cfg.Profile)
	}

	all, _ := session.List()
	raw, err := os.ReadFile(all[0].MetadataPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sk-or-child") {
		t.Fatalf("key value leaked into metadata:\n%s", raw)
	}
}
