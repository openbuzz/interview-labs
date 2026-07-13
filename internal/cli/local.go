package cli

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/kindx"
	"github.com/openbuzz/interview-labs/internal/pack"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/stack"
)

// localPassword is the fixed gateway password for local sessions: the
// gateway binds loopback-only, so a per-session secret buys nothing.
const localPassword = "openbuzz"

// localURL is where the local gateway answers.
const localURL = "http://localhost:8080"

// liveLocalSession returns the existing local session, if any — the
// gateway port is fixed, so one local stack runs at a time.
func liveLocalSession() (*session.Session, error) {
	all, err := session.List()
	if err != nil {
		return nil, err
	}
	for _, s := range all {
		if isLocalSession(s) {
			return s, nil
		}
	}
	return nil, nil
}

// runLocalLaunch drives the docker-on-this-machine pipeline: stage, mint,
// compose up. No terraform, no billing gate, no ssh.
func runLocalLaunch(cmd *cobra.Command, out io.Writer, cfg config.Config,
	sel provider.Provider, profile string, f *launchFlags,
	p *pack.Pack, bundle *pack.Bundle) error {
	existing, err := liveLocalSession()
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("local session %s already exists — one local stack at a "+
			"time (interview destroy %s)", existing.Meta.Slug, existing.Meta.Slug)
	}
	if err := gateKindHostBins(bundle); err != nil {
		return err
	}

	ai := activeAI(cfg, f.noAI || !isAIProfile(profile))
	s, err := newLocalSession(sel, profile, ai, p, bundle)
	if err != nil {
		return err
	}
	release, err := s.Lock()
	if err != nil {
		return err
	}
	defer release()

	images, err := resolveAndValidateImages(out, f, profile)
	if err != nil {
		return failLaunch(out, s, err)
	}
	if err := runLocalStack(cmd.Context(), out, cfg, ai, images,
		packOrNilFS(p), bundle, s); err != nil {
		return failLaunch(out, s, err)
	}
	return nil
}

// gateKindHostBins requires kind + kubectl on this machine before a kind
// bundle launches — checked before session.New so a failed gate leaves no
// session behind.
func gateKindHostBins(bundle *pack.Bundle) error {
	if bundle == nil || !bundle.HasKind {
		return nil
	}
	if err := kindx.HostBinsPresent(); err != nil {
		return fmt.Errorf("%w — local kubernetes bundles need kind and kubectl "+
			"on this machine (brew install kind kubectl)", err)
	}
	return nil
}

// newLocalSession creates the session and records the picked bundle, if
// any — mirrors newLaunchSession's cloud-path shape.
func newLocalSession(sel provider.Provider, profile string, ai provider.AI,
	p *pack.Pack, bundle *pack.Bundle) (*session.Session, error) {
	roles := map[string]string{"vm": sel.Name()}
	if ai != nil {
		roles["ai"] = ai.Name()
	}
	s, err := session.New("", "", "", "", roles, session.TerraformInfo{})
	if err != nil {
		return nil, err
	}
	s.Meta.Profile, s.Meta.GatewayPassword = profile, localPassword
	if bundle != nil {
		s.Meta.Pack, s.Meta.Bundle, s.Meta.Kind = p.Name, bundle.Name, bundle.HasKind
	}
	return s, s.Save()
}

// runLocalStack runs the local phases; step rows mirror the cloud pipeline.
func runLocalStack(ctx context.Context, out io.Writer, cfg config.Config,
	ai provider.AI, images resolvedImages, packFS fs.FS, bundle *pack.Bundle,
	s *session.Session) error {
	quiet := quietOutput()
	if err := step(out, quiet, "stage", func() error {
		return localStage(s, packFS, bundle)
	}); err != nil {
		return err
	}

	if bundle != nil && bundle.HasKind {
		if err := step(out, quiet, "create cluster", func() error {
			return localCreateCluster(ctx, out, quiet, s)
		}); err != nil {
			return err
		}
	}

	aiKey := ""
	if ai != nil {
		if err := step(out, quiet, "mint ai key", func() error {
			var mintErr error
			aiKey, mintErr = mintAIKey(ctx, ai, cfg, s)
			return mintErr
		}); err != nil {
			return err
		}
	}

	if err := step(out, quiet, "compose up", func() error {
		return localComposeUp(ctx, out, quiet, images, s, aiKey)
	}); err != nil {
		return err
	}

	if err := runLocalSetup(ctx, out, quiet, bundle, s); err != nil {
		return err
	}
	return printLaunchSummary(out, s)
}

// localStage materializes the compose file and, when a bundle is picked,
// the payload tree plus the compose override.
func localStage(s *session.Session, packFS fs.FS, bundle *pack.Bundle) error {
	if err := s.SetPhase("stage"); err != nil {
		return err
	}
	if err := stack.Stage(s.StackDir()); err != nil {
		return err
	}
	if bundle == nil {
		return nil
	}
	if err := pack.StagePayload(s.StackDir(), packFS, bundle); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.StackDir(), "override.yaml"),
		[]byte(stack.Override(true, bundle.HasLab, bundle.HasKind)), 0o644)
}

// localCreateCluster stands the session cluster up on this machine's
// docker; the admin kubeconfig stays session-scoped so ~/.kube/config is
// never touched.
func localCreateCluster(ctx context.Context, out io.Writer, quiet bool,
	s *session.Session) error {
	if err := s.SetPhase("create-cluster"); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(s.LogsDir(), "kind-create.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	dest := io.Writer(logFile)
	if !quiet {
		dest = io.MultiWriter(out, logFile)
	}

	payload := filepath.Join(s.StackDir(), "payload")
	return kindx.CreateLocal(ctx, s.Meta.Slug,
		filepath.Join(payload, "kind", "cluster.yaml"),
		filepath.Join(payload, "kind", "manifests"),
		filepath.Join(payload, "kubeconfig.admin"),
		filepath.Join(payload, "kubeconfig"), dest)
}

// localComposeUp starts the stack on the host engine; env vars carry the
// per-mode deltas (no GATEWAY_BIND — the compose default is loopback).
// compose up's default "missing" pull policy fetches absent images and uses
// present ones as-is — exactly what --tag local needs.
func localComposeUp(ctx context.Context, out io.Writer, quiet bool,
	images resolvedImages, s *session.Session, aiKey string) error {
	if err := s.SetPhase("compose-up"); err != nil {
		return err
	}
	env := []string{
		"GATEWAY_PASSWORD=" + localPassword,
		"GATEWAY_IMAGE=" + images.Gateway,
		"VSCODE_IMAGE=" + images.Vscode,
	}
	if aiKey != "" {
		env = append(env, "OPENROUTER_API_KEY="+aiKey)
	}

	args := []string{"compose"}
	if s.Meta.Bundle != "" {
		args = append(args, "-f", "compose.yaml", "-f", "override.yaml")
	}
	args = append(args, "-p", "interview-"+s.Meta.Slug, "up", "-d", "--wait")
	if err := execDocker(ctx, out, quiet, s, "stack-up.log", s.StackDir(), env,
		args...); err != nil {
		return err
	}
	return s.SetURL(localURL)
}

// runLocalSetup runs the bundle's setup.sh step when the bundle has one,
// exec'ing into the vscode container over the host docker CLI — the local
// mirror of the cloud runCloudSetup phase; split out of runLocalStack (guard
// included) to stay under the complexity gate.
func runLocalSetup(ctx context.Context, out io.Writer, quiet bool, bundle *pack.Bundle,
	s *session.Session) error {
	if bundle == nil || !bundle.HasSetup {
		return nil
	}
	return step(out, quiet, "run setup.sh", func() error {
		if err := s.SetPhase("setup"); err != nil {
			return err
		}
		return execDocker(ctx, out, quiet, s, "setup.log", s.StackDir(), nil,
			"compose", "-f", "compose.yaml", "-f", "override.yaml",
			"-p", "interview-"+s.Meta.Slug, "exec", "-T",
			"-e", "INTERVIEW_SESSION_ID="+s.Meta.Slug,
			"-e", "INTERVIEW_BUNDLE="+bundle.Name,
			"-e", "INTERVIEW_SCENARIOS="+scenarioCSV(bundle),
			"-e", "INTERVIEW_LAB_DIR=/opt/interview/lab",
			"vscode", "bash", "/opt/interview/lab/setup.sh")
	})
}

// execDocker runs one docker command, teeing output to the session log
// (and the terminal when verbose). dir may be empty for cwd-independent
// commands like compose down.
func execDocker(ctx context.Context, out io.Writer, quiet bool, s *session.Session,
	logName, dir string, extraEnv []string, args ...string) error {
	logPath := filepath.Join(s.LogsDir(), logName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	dest := io.Writer(logFile)
	if !quiet {
		dest = io.MultiWriter(out, logFile)
	}

	c := exec.CommandContext(ctx, "docker", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), extraEnv...)
	c.Stdout, c.Stderr = dest, dest
	if err := c.Run(); err != nil {
		return fmt.Errorf("docker %s failed (log: %s): %w", args[0], logPath, err)
	}
	return nil
}
