// Package kindx drives session kind clusters by exec'ing the kind and
// kubectl binaries — the host's for local sessions, the VM's (installed by
// InstallScript) over ssh for cloud sessions. No k8s client libraries.
package kindx

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// Pinned toolchain for the VM install; sha256-verified per arch. Floors for
// the operator-host check derive from the same families.
const (
	KindVersion    = "0.32.0"
	KubectlVersion = "1.36.2"
)

// InstallScript installs kind + kubectl on the session VM, idempotently.
// Runs via `bash -s` with this script on stdin.
const InstallScript = `#!/usr/bin/env bash

set -e -u -o pipefail

KUBECTL_VERSION="1.36.2"
KIND_VERSION="0.32.0"

ARCH="$(dpkg --print-architecture)"

if ! command -v kubectl >/dev/null 2>&1; then
  case "${ARCH}" in
  amd64) sha="1e9045ec32bea85da43de85f0065358529ea7c7a152eca78154fba5b58c27d82" ;;
  arm64) sha="c957eb8c4bea27a3bb35b269edd9082e27f027f7b76b20b5bf4afebc726c6d3e" ;;
  *)
    echo "unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
  esac
  curl -fsSL -o /tmp/kubectl \
    "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl"
  printf '%s  /tmp/kubectl\n' "${sha}" | sha256sum -c -
  install -m 0755 /tmp/kubectl /usr/local/bin/kubectl
  rm -f /tmp/kubectl
fi

if ! command -v kind >/dev/null 2>&1; then
  case "${ARCH}" in
  amd64) sha="50030de23cf40a18505f20426f6a8506bedf13c6e509244bd1fa9463721b0f54" ;;
  arm64) sha="b92cd615e97585de8ddade28ed5cd7feb4248d717c233eea5b03c37298900f5d" ;;
  *)
    echo "unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
  esac
  curl -fsSL -o /tmp/kind \
    "https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-linux-${ARCH}"
  printf '%s  /tmp/kind\n' "${sha}" | sha256sum -c -
  install -m 0755 /tmp/kind /usr/local/bin/kind
  rm -f /tmp/kind
fi
`

// ClusterName names a session's cluster.
func ClusterName(slug string) string { return "interview-" + slug }

// Remote (VM) command builders — payload paths per stack.RemoteDir layout.

// CreateClusterCmd creates the cluster from the pushed pack config.
func CreateClusterCmd(slug string) string {
	return "kind create cluster --name " + ClusterName(slug) +
		" --config /opt/interview/docker/payload/kind/cluster.yaml --wait 60s"
}

// ApplyManifestsCmd applies the pack manifests in name order; no health
// wait — troubleshooting packs deploy deliberately broken workloads.
func ApplyManifestsCmd() string {
	return `for f in /opt/interview/docker/payload/kind/manifests/*.yaml; do ` +
		`kubectl apply -f "$f" || exit 1; done`
}

// WriteKubeconfigCmd writes the docker-network-reachable kubeconfig where
// the vscode container mounts it, owned by the candidate uid.
func WriteKubeconfigCmd(slug string) string {
	return "kind get kubeconfig --name " + ClusterName(slug) +
		" --internal > /opt/interview/docker/payload/kubeconfig && " +
		"chown 1000:1000 /opt/interview/docker/payload/kubeconfig"
}

// HostBinsPresent reports whether the operator host has kind and kubectl.
func HostBinsPresent() error {
	for _, bin := range []string{"kind", "kubectl"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found on PATH", bin)
		}
	}
	return nil
}

// CreateLocal creates the session cluster on the operator's docker: create
// (scoped to its own admin kubeconfig — the operator's ~/.kube/config is
// never touched), apply manifests sorted, write the internal kubeconfig.
func CreateLocal(ctx context.Context, slug, clusterYAML, manifestsDir,
	kubeconfigAdmin, kubeconfigInternal string, logW io.Writer) error {
	if err := runLocal(ctx, logW, nil, "kind", "create", "cluster",
		"--name", ClusterName(slug), "--config", clusterYAML,
		"--kubeconfig", kubeconfigAdmin, "--wait", "60s"); err != nil {
		return err
	}

	manifests, err := filepath.Glob(filepath.Join(manifestsDir, "*.yaml"))
	if err != nil {
		return err
	}
	sort.Strings(manifests)
	for _, m := range manifests {
		if err := runLocal(ctx, logW, nil, "kubectl",
			"--kubeconfig", kubeconfigAdmin, "apply", "-f", m); err != nil {
			return err
		}
	}

	internal, err := os.Create(kubeconfigInternal)
	if err != nil {
		return err
	}
	defer internal.Close()
	return runLocal(ctx, logW, internal, "kind", "get", "kubeconfig",
		"--name", ClusterName(slug), "--internal")
}

// DeleteLocal removes the session cluster; an absent cluster is fine —
// failed-destroy reruns must stay idempotent.
func DeleteLocal(ctx context.Context, slug string, logW io.Writer) error {
	// kind delete exits zero for unknown clusters on current versions;
	// a non-zero here is a real failure.
	return runLocal(ctx, logW, nil, "kind", "delete", "cluster",
		"--name", ClusterName(slug))
}

// runLocal execs one binary, teeing combined output to logW (and stdout to
// stdoutW when non-nil — kubeconfig capture).
func runLocal(ctx context.Context, logW io.Writer, stdoutW io.Writer,
	bin string, args ...string) error {
	c := exec.CommandContext(ctx, bin, args...)
	if stdoutW != nil {
		c.Stdout, c.Stderr = stdoutW, logW
	} else {
		c.Stdout, c.Stderr = logW, logW
	}
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s %v failed: %w", bin, args, err)
	}
	return nil
}
