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
)

// runCmd executes the root command with args, capturing stdout+stderr.
func runCmd(t *testing.T, args ...string) (string, int) {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	code := 0
	if err := root.Execute(); err != nil {
		fmt.Fprintf(&buf, "error: %v\n", err)
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
	old := validateCreds
	validateCreds = func(context.Context, provider.VM, config.Config) error { return nil }
	t.Cleanup(func() { validateCreds = old })

	out, code := runCmd(t, "doctor")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	for _, want := range []string{"terraform", "1.9.5", "DigitalOcean", "credentials valid"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
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

	out, code := runCmd(t, "doctor")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "✗") {
		t.Fatalf("no fail glyph:\n%s", out)
	}
	for _, want := range []string{"not configured", "run interview init"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
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
	oldValidate := validateCreds
	validateCreds = func(context.Context, provider.VM, config.Config) error { return nil }
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
	old := validateCreds
	validateCreds = func(context.Context, provider.VM, config.Config) error {
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
	old := validateCreds
	validateCreds = func(context.Context, provider.VM, config.Config) error { return nil }
	t.Cleanup(func() { validateCreds = old })

	out, code := runCmd(t, "doctor")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "state dirs") {
		t.Fatalf("missing state dirs row:\n%s", out)
	}
}
