// Package localvm is the pseudo-provider that runs the interview stack on
// the operator's own docker engine instead of a cloud VM. It serves the vm
// role but deliberately does not implement provider.VM — no terraform, no
// regions, no billing — and launch dispatches on that absence.
package localvm

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

const guidanceTitle = "Local Docker sessions"

const guidance = `Runs the interview stack in containers on this machine —
no cloud account, no billing, no ssh.

Requirements:
1. A running docker engine (Docker Desktop, OrbStack, or docker-ce)
2. Roughly 10 GB of free disk for the built images

Nothing is stored in the config; launch probes the daemon each run.`

// Seams for tests.
var (
	out io.Writer = os.Stdout

	lookDocker = func() error {
		_, err := exec.LookPath("docker")
		return err
	}
	dockerInfo = func(ctx context.Context) error {
		return exec.CommandContext(ctx, "docker", "info").Run()
	}
)

type lv struct{}

// New returns the local pseudo-provider.
func New() provider.Provider { return lv{} }

func (lv) Name() string  { return "local" }
func (lv) Label() string { return "Local Docker" }

func (lv) Roles() []provider.Role { return []provider.Role{provider.RoleVM} }

// Configured reports whether a docker CLI is on PATH; the daemon probe
// lives in ValidateCreds so menus stay instant.
func (lv) Configured(config.Config) bool { return lookDocker() == nil }

// Configure has nothing to store: it explains the requirement and probes
// the daemon once for immediate feedback.
func (l lv) Configure(ctx context.Context, _ *config.Config) error {
	fmt.Fprintln(out, ui.Box(guidanceTitle, ui.Accent, strings.Split(guidance, "\n")...))

	if err := provider.TestCredentials(ctx, out, ui.Step, func(ctx context.Context) error {
		return l.ValidateCreds(ctx, config.Config{})
	}); err != nil {
		fmt.Fprintln(out, ui.RowFail("docker", err.Error()))
		return nil
	}
	fmt.Fprintln(out, ui.RowOK("docker", "daemon reachable"))
	return nil
}

// ValidateCreds probes the docker daemon with one cheap call.
func (lv) ValidateCreds(ctx context.Context, _ config.Config) error {
	if err := lookDocker(); err != nil {
		return fmt.Errorf("docker CLI not found — install docker to use local sessions")
	}
	if err := dockerInfo(ctx); err != nil {
		return fmt.Errorf("docker daemon unreachable — is docker running?")
	}
	return nil
}
