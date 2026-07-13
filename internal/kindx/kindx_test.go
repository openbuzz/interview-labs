package kindx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClusterName(t *testing.T) {
	if got := ClusterName("calm-otter"); got != "interview-calm-otter" {
		t.Fatalf("ClusterName = %q", got)
	}
}

func TestRemoteCmds(t *testing.T) {
	create := CreateClusterCmd("calm-otter")
	for _, want := range []string{
		"kind create cluster", "--name interview-calm-otter",
		"--config /opt/interview/docker/payload/kind/cluster.yaml", "--wait 60s",
	} {
		if !strings.Contains(create, want) {
			t.Fatalf("create missing %q: %q", want, create)
		}
	}
	if !strings.Contains(ApplyManifestsCmd(),
		"/opt/interview/docker/payload/kind/manifests/*.yaml") {
		t.Fatalf("apply = %q", ApplyManifestsCmd())
	}
	kc := WriteKubeconfigCmd("calm-otter")
	for _, want := range []string{"--internal", "/opt/interview/docker/payload/kubeconfig",
		"chown 1000:1000"} {
		if !strings.Contains(kc, want) {
			t.Fatalf("kubeconfig cmd missing %q: %q", want, kc)
		}
	}
}

func TestInstallScriptPins(t *testing.T) {
	for _, want := range []string{
		"KUBECTL_VERSION=\"" + KubectlVersion + "\"",
		"KIND_VERSION=\"" + KindVersion + "\"",
		"sha256sum -c -",
	} {
		if !strings.Contains(InstallScript, want) {
			t.Fatalf("install script missing %q", want)
		}
	}
}

// fakeBins installs logging kind/kubectl fakes on PATH and returns the log.
func fakeBins(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\necho \"${0##*/} $@\" >> " + log + "\nexit 0\n"
	for _, name := range []string{"kind", "kubectl"} {
		if err := os.WriteFile(filepath.Join(dir, name),
			[]byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return log
}

//nolint:cyclop // cyclop: dense field checks over one fixture — a split wouldn't lower it
func TestCreateLocalSequence(t *testing.T) {
	log := fakeBins(t)
	dir := t.TempDir()
	cluster := filepath.Join(dir, "cluster.yaml")
	manifests := filepath.Join(dir, "manifests")
	if err := os.WriteFile(cluster, []byte("kind: Cluster\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(manifests, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"10-b.yaml", "00-a.yaml"} {
		if err := os.WriteFile(filepath.Join(manifests, m),
			[]byte("apiVersion: v1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := CreateLocal(context.Background(), "calm-otter", cluster, manifests,
		filepath.Join(dir, "kc.admin"), filepath.Join(dir, "kc.internal"), os.Stderr)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(log)
	calls := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(calls) != 4 {
		t.Fatalf("calls = %q", calls)
	}
	if !strings.Contains(calls[0], "kind create cluster") ||
		!strings.Contains(calls[0], "--name interview-calm-otter") {
		t.Fatalf("call 0 = %q", calls[0])
	}
	if !strings.Contains(calls[1], "00-a.yaml") || !strings.Contains(calls[2], "10-b.yaml") {
		t.Fatalf("manifests out of order: %q", calls)
	}
	if !strings.Contains(calls[3], "kind get kubeconfig") ||
		!strings.Contains(calls[3], "--internal") {
		t.Fatalf("call 3 = %q", calls[3])
	}
	if _, err := os.Stat(filepath.Join(dir, "kc.internal")); err != nil {
		t.Fatalf("internal kubeconfig not written: %v", err)
	}
}

func TestDeleteLocal(t *testing.T) {
	log := fakeBins(t)
	if err := DeleteLocal(context.Background(), "calm-otter", os.Stderr); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "kind delete cluster --name interview-calm-otter") {
		t.Fatalf("delete call = %q", string(data))
	}
}
