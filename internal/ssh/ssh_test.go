package ssh

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDialRunAndPin(t *testing.T) {
	addr, privPEM, _ := StartTestServer(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key")
	if err := os.WriteFile(keyPath, []byte(privPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	knownHosts := filepath.Join(dir, "known_hosts")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := Dial(ctx, addr, "root", keyPath, knownHosts)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	out, err := c.Run(ctx, "echo 'Hello world'")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hello world") {
		t.Fatalf("Run output = %q", out)
	}

	pinned, err := os.ReadFile(knownHosts)
	if err != nil || len(pinned) == 0 {
		t.Fatalf("known_hosts not pinned: %v", err)
	}

	// Second dial must verify against the pin and still work.
	c2, err := Dial(ctx, addr, "root", keyPath, knownHosts)
	if err != nil {
		t.Fatalf("second dial against pin: %v", err)
	}
	c2.Close()
}

func TestDialRejectsChangedHostKey(t *testing.T) {
	addr, privPEM, _ := StartTestServer(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key")
	if err := os.WriteFile(keyPath, []byte(privPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	knownHosts := filepath.Join(dir, "known_hosts")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := Dial(ctx, addr, "root", keyPath, knownHosts)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	// A different server (new host key) on a fresh port, pinned entry rewritten to
	// claim that address — dial must fail fast, not retry forever.
	addr2, _, _ := StartTestServer(t)
	pinned, err := os.ReadFile(knownHosts)
	if err != nil {
		t.Fatal(err)
	}
	swapped := strings.ReplaceAll(string(pinned), hostPart(addr), hostPart(addr2))
	if err := os.WriteFile(knownHosts, []byte(swapped), 0o600); err != nil {
		t.Fatal(err)
	}

	shortCtx, cancel2 := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel2()
	if _, err := Dial(shortCtx, addr2, "root", keyPath, knownHosts); err == nil {
		t.Fatal("dial succeeded against changed host key")
	}
}

// hostPart normalizes an addr the way known_hosts stores it ([host]:port).
func hostPart(addr string) string { return "[" + strings.Replace(addr, ":", "]:", 1) }

func TestArgv(t *testing.T) {
	got := Argv("/s/ssh/key", "/s/ssh/known_hosts", "root", "203.0.113.9")
	want := []string{"ssh", "-i", "/s/ssh/key",
		"-o", "UserKnownHostsFile=/s/ssh/known_hosts", "root@203.0.113.9"}
	if len(got) != len(want) {
		t.Fatalf("Argv() = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Argv()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// dialTestServer wires a client to the recording server with a fresh
// key/known_hosts pair in a temp dir.
func dialTestServer(t *testing.T) (*Client, *ExecRecorder) {
	t.Helper()
	addr, privPEM, _, rec := StartRecordingTestServer(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key")
	if err := os.WriteFile(keyPath, []byte(privPEM), 0o600); err != nil {
		t.Fatal(err)
	}

	client, err := Dial(context.Background(), addr, "test", keyPath,
		filepath.Join(dir, "known_hosts"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client, rec
}

func TestRunInRecordsCommandAndDrainsStdin(t *testing.T) {
	client, rec := dialTestServer(t)

	payload := bytes.Repeat([]byte("x"), 1<<20) // 1 MiB: stalls if undrained
	out, err := client.RunIn(context.Background(), bytes.NewReader(payload),
		"tar -xzf - -C /opt/interview")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "Hello world") {
		t.Fatalf("output = %q", out)
	}
	cmds := rec.Commands()
	if len(cmds) != 1 || cmds[0] != "tar -xzf - -C /opt/interview" {
		t.Fatalf("recorded = %v", cmds)
	}
}

func TestRunStreamWritesOutput(t *testing.T) {
	client, rec := dialTestServer(t)

	var buf bytes.Buffer
	if err := client.RunStream(context.Background(), &buf,
		"docker buildx bake gateway devops"); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "Hello world") {
		t.Fatalf("streamed = %q", buf.String())
	}
	if got := rec.Commands(); len(got) != 1 ||
		got[0] != "docker buildx bake gateway devops" {
		t.Fatalf("recorded = %v", got)
	}
}
