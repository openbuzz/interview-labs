package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// runCmd executes the root command with args, capturing stdout+stderr.
func runCmd(t *testing.T, args ...string) (string, int) {
	t.Helper()
	ui.ResetLogoOnce()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	code := 0
	if err := root.Execute(); err != nil {
		if !isSilenced(err) {
			fmt.Fprintf(&buf, "error: %v\n", err)
		}
		if IsUsage(err) || isCobraUsage(err) {
			code = 2
		} else {
			code = 1
		}
	}
	return buf.String(), code
}

// TestRunCmdMapsCobraUsageToExit2 guards runCmd against drifting from run()'s
// exit-code mapping in root.go (IsUsage || isCobraUsage).
func TestRunCmdMapsCobraUsageToExit2(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	out, code := runCmd(t, "definitely-not-a-command")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\n%s", code, out)
	}
}

// fakeTF puts a fake terraform binary on PATH (and nothing else but sh).
func fakeTF(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"[ \"$1\" = version ] && echo '{\"terraform_version\":\"1.9.5\"}'\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// keep sh reachable for the fake script itself
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+
		string(os.PathListSeparator)+"/usr/bin")
}

func TestDoctorAllGood(t *testing.T) {
	fakeTF(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	old := validateCreds
	validateCreds = func(context.Context, provider.CredentialValidator,
		config.Config) error {
		return nil
	}
	t.Cleanup(func() { validateCreds = old })

	out, code := runCmd(t, "doctor")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	for _, want := range []string{
		"TOOLS", "CREDENTIALS", "terraform", "1.9.5", "DigitalOcean",
		"credentials valid", "OpenRouter", "Cloudflare", "not configured",
		"ready", "interview launch",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "state dirs") {
		t.Fatalf("healthy state dirs row must be hidden:\n%s", out)
	}
}

func TestDoctorNoTFBinaryFails(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // nothing on PATH at all
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")

	out, code := runCmd(t, "doctor")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "✗") {
		t.Fatalf("no fail glyph:\n%s", out)
	}
	for _, want := range []string{"not configured", "interview init", "1 problem"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "error:") {
		t.Fatalf("silenced doctor error still printed:\n%s", out)
	}
}

func TestDoctorMissingSSHIsNote(t *testing.T) {
	fakeTF(t)
	old := lookupSSH
	lookupSSH = func() error { return errors.New("not found") }
	t.Cleanup(func() { lookupSSH = old })
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	oldValidate := validateCreds
	validateCreds = func(context.Context, provider.CredentialValidator,
		config.Config) error {
		return nil
	}
	t.Cleanup(func() { validateCreds = oldValidate })

	out, code := runCmd(t, "doctor")
	if code != 0 {
		t.Fatalf("missing ssh must not fail doctor; exit = %d\n%s", code, out)
	}
	if !strings.Contains(out, "▲") {
		t.Fatalf("expected warn glyph for ssh row:\n%s", out)
	}
}

func TestDoctorRejectedTokenFails(t *testing.T) {
	fakeTF(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "bad")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	old := validateCreds
	validateCreds = func(context.Context, provider.CredentialValidator,
		config.Config) error {
		return errors.New("token rejected by DigitalOcean")
	}
	t.Cleanup(func() { validateCreds = old })

	out, code := runCmd(t, "doctor")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "rejected") {
		t.Fatalf("missing rejection detail:\n%s", out)
	}
	for _, want := range []string{"1 problem", "interview init"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorXDGUnwritableDirFails(t *testing.T) {
	fakeTF(t)
	roDir := t.TempDir()
	if err := os.Chmod(roDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0o700) })

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(roDir, "config"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	old := validateCreds
	validateCreds = func(context.Context, provider.CredentialValidator,
		config.Config) error {
		return nil
	}
	t.Cleanup(func() { validateCreds = old })

	out, code := runCmd(t, "doctor")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "state dirs") {
		t.Fatalf("missing state dirs row:\n%s", out)
	}
}

func TestDoctorChecksAIAndAccessProviders(t *testing.T) {
	fakeTF(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cf-tok")

	cfg := config.Config{}
	cfg.Providers.Cloudflare.ZoneID = "z1"
	cfg.Providers.Cloudflare.Domain = "example.test"
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}

	var checked []string
	old := validateCreds
	validateCreds = func(_ context.Context, v provider.CredentialValidator,
		_ config.Config) error {
		if p, ok := v.(provider.Provider); ok {
			checked = append(checked, p.Name())
		}
		return nil
	}
	t.Cleanup(func() { validateCreds = old })

	out, code := runCmd(t, "doctor")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	want := []string{"openrouter", "cloudflare"}
	for _, w := range want {
		found := false
		for _, c := range checked {
			found = found || c == w
		}
		if !found {
			t.Fatalf("doctor never validated %s (checked %v)", w, checked)
		}
	}
}

func TestDoctorKindRowsMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir()) // nothing on PATH

	out, _ := runCmd(t, "doctor")
	for _, want := range []string{"kind", "kubectl", "local kubernetes"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorKindVersionFloor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = fakeLocalBins(t, "kind", "kubectl") // present; version output is fake

	old := kindToolVersion
	kindToolVersion = func(bin string) (string, error) {
		if bin == "kind" {
			return "kind v0.20.0 go1.21 linux/amd64", nil // below the 0.32 floor
		}
		return "Client Version: v1.36.2", nil
	}
	t.Cleanup(func() { kindToolVersion = old })

	out, _ := runCmd(t, "doctor")
	if !strings.Contains(out, "below v0.32") {
		t.Fatalf("no floor warning:\n%s", out)
	}
	if !strings.Contains(out, "v1.36.2") {
		t.Fatalf("kubectl version row missing:\n%s", out)
	}
}

func TestDoctorVerdictPluralizes(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // terraform missing = problem 1
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "bad")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	old := validateCreds
	validateCreds = func(context.Context, provider.CredentialValidator,
		config.Config) error {
		return errors.New("rejected") // problem 2
	}
	t.Cleanup(func() { validateCreds = old })

	out, code := runCmd(t, "doctor")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "2 problems") {
		t.Fatalf("verdict not pluralized:\n%s", out)
	}
}

func TestDoctorSectionsOrderToolsFirst(t *testing.T) {
	fakeTF(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("HCLOUD_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	old := validateCreds
	validateCreds = func(context.Context, provider.CredentialValidator,
		config.Config) error {
		return nil
	}
	t.Cleanup(func() { validateCreds = old })

	out, _ := runCmd(t, "doctor")
	tools, creds := strings.Index(out, "TOOLS"), strings.Index(out, "CREDENTIALS")
	if tools == -1 || creds == -1 || tools > creds {
		t.Fatalf("sections wrong or missing (TOOLS@%d, CREDENTIALS@%d):\n%s",
			tools, creds, out)
	}
	if strings.Index(out, "terraform") > strings.Index(out, "DigitalOcean") {
		t.Fatalf("tool row after credential row:\n%s", out)
	}
}
