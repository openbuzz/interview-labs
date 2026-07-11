package terraform

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRunner returns a Runner whose binary records argv and env to files.
func fakeRunner(t *testing.T) (*Runner, string) {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> " + record + "\n" +
		"echo \"token=$DIGITALOCEAN_TOKEN cache=$TF_PLUGIN_CACHE_DIR\" >> " + record + "\n" +
		"if [ \"$1\" = \"output\" ]; then cat " + dir + "/outputs.json; fi\n" +
		"echo fake-stdout\n"
	bin := filepath.Join(dir, "terraform")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	work := filepath.Join(dir, "work")
	logs := filepath.Join(dir, "logs")
	for _, d := range []string{work, logs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := &Runner{
		Bin: Binary{Name: "terraform", Path: bin, Version: "1.9.5"},
		Dir: work,
		Env: RunEnv(map[string]string{"DIGITALOCEAN_TOKEN": "tok-123"},
			filepath.Join(dir, "plugins")),
		LogsDir: logs,
		Out:     &bytes.Buffer{},
	}
	return r, record
}

func TestApplyArgsEnvAndLog(t *testing.T) {
	r, record := fakeRunner(t)
	if err := r.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rec), "apply -input=false -auto-approve") {
		t.Fatalf("argv: %s", rec)
	}
	if !strings.Contains(string(rec), "token=tok-123") {
		t.Fatalf("env not passed: %s", rec)
	}

	log, err := os.ReadFile(filepath.Join(r.LogsDir, "terraform-apply.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "--- ") || !strings.Contains(string(log), "fake-stdout") {
		t.Fatalf("log content: %s", log)
	}
}

func TestLogAppendsAcrossAttempts(t *testing.T) {
	r, _ := fakeRunner(t)
	if err := r.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(filepath.Join(r.LogsDir, "terraform-init.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(log), "--- "); got != 2 {
		t.Fatalf("separator count = %d, want 2", got)
	}
}

func TestRunEnvAppendsSortedCreds(t *testing.T) {
	env := RunEnv(map[string]string{
		"B_TOKEN": "b", "A_TOKEN": "a",
	}, "/cache")

	var got []string
	for _, e := range env {
		if e == "A_TOKEN=a" || e == "B_TOKEN=b" ||
			e == "TF_PLUGIN_CACHE_DIR=/cache" || e == "TF_IN_AUTOMATION=1" {
			got = append(got, e)
		}
	}
	want := []string{"A_TOKEN=a", "B_TOKEN=b", "TF_PLUGIN_CACHE_DIR=/cache",
		"TF_IN_AUTOMATION=1"}
	if len(got) != len(want) {
		t.Fatalf("env entries = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("env[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOutputsParse(t *testing.T) {
	r, _ := fakeRunner(t)
	outputs := `{"ip":{"value":"203.0.113.7"}}`
	if err := os.WriteFile(filepath.Join(filepath.Dir(r.Bin.Path), "outputs.json"),
		[]byte(outputs), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := r.Outputs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.IP != "203.0.113.7" {
		t.Fatalf("Outputs() = %+v", got)
	}
}

func TestOutputsParsesFQDN(t *testing.T) {
	r, _ := fakeRunner(t)
	outputs := `{"ip":{"value":"203.0.113.9"},"fqdn":{"value":"calm-otter.example.test"}}`
	if err := os.WriteFile(filepath.Join(filepath.Dir(r.Bin.Path), "outputs.json"),
		[]byte(outputs), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := r.Outputs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.IP != "203.0.113.9" || got.FQDN != "calm-otter.example.test" {
		t.Fatalf("Outputs() = %+v", got)
	}
}

func TestOutputsMissingFQDNIsEmpty(t *testing.T) {
	r, _ := fakeRunner(t)
	outputs := `{"ip":{"value":"203.0.113.9"}}`
	if err := os.WriteFile(filepath.Join(filepath.Dir(r.Bin.Path), "outputs.json"),
		[]byte(outputs), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := r.Outputs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.FQDN != "" {
		t.Fatalf("fqdn = %q, want empty", got.FQDN)
	}
}
