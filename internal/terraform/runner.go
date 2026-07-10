package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

// Runner executes tf commands in a session's terraform dir.
type Runner struct {
	Bin     Binary
	Dir     string
	Env     []string
	LogsDir string
	Out     io.Writer
}

// Outputs are the root module's output values.
type Outputs struct {
	IP string
}

// PluginCacheDir returns (and creates) the shared provider cache.
func PluginCacheDir() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(base, "interview", "terraform", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// RunEnv builds the child env: parent + provider creds + plugin cache +
// automation flag. Creds append in sorted key order so runs are
// reproducible.
func RunEnv(creds map[string]string, pluginCache string) []string {
	keys := make([]string, 0, len(creds))
	for k := range creds {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	env := os.Environ()
	for _, k := range keys {
		env = append(env, k+"="+creds[k])
	}
	return append(env,
		"TF_PLUGIN_CACHE_DIR="+pluginCache,
		"TF_IN_AUTOMATION=1",
	)
}

// Init runs `tf init`.
func (r *Runner) Init(ctx context.Context) error {
	return r.run(ctx, "init", "-input=false")
}

// Apply runs `tf apply`.
func (r *Runner) Apply(ctx context.Context) error {
	return r.run(ctx, "apply", "-input=false", "-auto-approve")
}

// Destroy runs `tf destroy`.
func (r *Runner) Destroy(ctx context.Context) error {
	return r.run(ctx, "destroy", "-input=false", "-auto-approve")
}

func (r *Runner) run(ctx context.Context, sub string, args ...string) error {
	logPath := filepath.Join(r.LogsDir, "terraform-"+sub+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "--- %s terraform %s ---\n",
		time.Now().UTC().Format(time.RFC3339), sub)

	sink := io.MultiWriter(r.Out, logFile)
	cmd := exec.Command(r.Bin.Path, append([]string{sub}, args...)...)
	cmd.Dir = r.Dir
	cmd.Env = r.Env
	cmd.Stdout = sink
	cmd.Stderr = sink
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s %s failed (log: %s): %w", r.Bin.Name, sub, logPath, err)
		}
		return nil
	case <-ctx.Done():
		syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
		return ctx.Err()
	}
}

// Outputs parses `tf output -json`.
func (r *Runner) Outputs(ctx context.Context) (Outputs, error) {
	cmd := exec.CommandContext(ctx, r.Bin.Path, "output", "-json")
	cmd.Dir = r.Dir
	cmd.Env = r.Env
	raw, err := cmd.Output()
	if err != nil {
		return Outputs{}, fmt.Errorf("%s output: %w", r.Bin.Name, err)
	}

	var parsed map[string]struct {
		Value string `json:"value"`
	}
	// Decode (not Unmarshal): stops after the first JSON value so trailing
	// stdout noise from the child process doesn't fail the parse.
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&parsed); err != nil {
		return Outputs{}, err
	}
	return Outputs{IP: parsed["ip"].Value}, nil
}
