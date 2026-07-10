package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
)

func TestInitNonTTYExits2(t *testing.T) {
	old := isTTY
	isTTY = func() bool { return false }
	t.Cleanup(func() { isTTY = old })

	out, code := runCmd(t, "init")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "config.yaml") {
		t.Fatalf("non-TTY message must point at the config file:\n%s", out)
	}
}

func TestInitValidatesThenWrites(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldTTY, oldPrompt, oldValidate := isTTY, promptToken, validateDOToken
	isTTY = func() bool { return true }
	promptToken = func(validate func(string) error) (string, error) {
		if err := validate("dop_v1_good"); err != nil {
			t.Fatalf("validator rejected: %v", err)
		}
		return "dop_v1_good", nil
	}
	validateDOToken = func(ctx context.Context, token string) error { return nil }
	t.Cleanup(func() { isTTY, promptToken, validateDOToken = oldTTY, oldPrompt, oldValidate })

	out, code := runCmd(t, "init")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	c, err := config.Load()
	if err != nil || c.DigitalOceanToken != "dop_v1_good" {
		t.Fatalf("config after init = %+v, %v", c, err)
	}
	for _, want := range []string{"cloud.digitalocean.com", "NEXT", "interview launch"} {
		if !strings.Contains(out, want) {
			t.Fatalf("init output missing %q:\n%s", want, out)
		}
	}
}
