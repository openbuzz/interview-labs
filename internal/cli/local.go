package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
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

// runLocalLaunch drives the docker-on-this-machine pipeline: stage, build,
// mint, compose up. No terraform, no billing gate, no ssh.
func runLocalLaunch(cmd *cobra.Command, out io.Writer, cfg config.Config,
	sel provider.Provider, profile string, noAI bool) error {
	existing, err := liveLocalSession()
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("local session %s already exists — one local stack at a "+
			"time (interview destroy %s)", existing.Meta.Slug, existing.Meta.Slug)
	}

	ai := activeAI(cfg, noAI || !isAIProfile(profile))
	roles := map[string]string{"vm": sel.Name()}
	if ai != nil {
		roles["ai"] = ai.Name()
	}
	s, err := session.New("", "", "", "", roles, session.TerraformInfo{})
	if err != nil {
		return err
	}
	s.Meta.Profile, s.Meta.GatewayPassword = profile, localPassword
	if err := s.Save(); err != nil {
		return err
	}
	release, err := s.Lock()
	if err != nil {
		return err
	}
	defer release()

	if err := runLocalStack(cmd.Context(), out, cfg, ai, s); err != nil {
		return failLaunch(out, s, err)
	}
	return nil
}

// runLocalStack runs the local phases; step rows mirror the cloud pipeline.
func runLocalStack(ctx context.Context, out io.Writer, cfg config.Config,
	ai provider.AI, s *session.Session) error {
	quiet := quietOutput()
	if err := step(out, quiet, "stage", func() error {
		if err := s.SetPhase("stage"); err != nil {
			return err
		}
		return stack.Stage(s.StackDir())
	}); err != nil {
		return err
	}
	if err := step(out, quiet, "build stack", func() error {
		if err := s.SetPhase("build-stack"); err != nil {
			return err
		}
		return execDocker(ctx, out, quiet, s, "stack-build.log", s.StackDir(), nil,
			"buildx", "bake", "gateway", s.Meta.Profile)
	}); err != nil {
		return err
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
		return localComposeUp(ctx, out, quiet, s, aiKey)
	}); err != nil {
		return err
	}
	return printLaunchSummary(out, s)
}

// localComposeUp starts the stack on the host engine; env vars carry the
// per-mode deltas (no GATEWAY_BIND — the compose default is loopback).
func localComposeUp(ctx context.Context, out io.Writer, quiet bool,
	s *session.Session, aiKey string) error {
	if err := s.SetPhase("compose-up"); err != nil {
		return err
	}
	env := []string{
		"GATEWAY_PASSWORD=" + localPassword,
		"VSCODE_PROFILE=" + s.Meta.Profile,
	}
	if aiKey != "" {
		env = append(env, "OPENROUTER_API_KEY="+aiKey)
	}

	if err := execDocker(ctx, out, quiet, s, "stack-up.log", s.StackDir(), env,
		"compose", "-p", "interview-"+s.Meta.Slug, "up", "-d", "--wait"); err != nil {
		return err
	}
	return s.SetURL(localURL)
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
