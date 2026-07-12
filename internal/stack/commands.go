package stack

import (
	"fmt"
	"sort"
	"strings"
)

// RemoteDir is where the payload lands on the VM; the stack itself lives
// in RemoteDir/docker.
const RemoteDir = "/opt/interview"

// ComposeUpCmd starts the stack. Env arrives by sourcing stdin (EnvBlob),
// never argv — secrets stay out of /proc cmdlines.
func ComposeUpCmd(slug string) string {
	return "cd " + RemoteDir + "/docker && set -a && . /dev/stdin && set +a && " +
		"docker compose -p interview-" + slug + " up -d --wait"
}

// PushCmd receives the compose file arriving on stdin.
func PushCmd() string {
	return "mkdir -p " + RemoteDir + "/docker && cat > " + RemoteDir +
		"/docker/compose.yaml"
}

// PullCmd pulls the session's two images. Refs cross as inline env (public
// registry names charset-validated at resolve time, not secrets) because
// compose interpolates GATEWAY_IMAGE/VSCODE_IMAGE at parse time — pull and
// up must see the same values.
func PullCmd(gatewayRef, vscodeRef string) string {
	return "cd " + RemoteDir + "/docker && GATEWAY_IMAGE='" + gatewayRef +
		"' VSCODE_IMAGE='" + vscodeRef + "' docker compose pull"
}

// EnvBlob renders KEY='value' lines sorted by key, single quotes escaped,
// for ComposeUpCmd's stdin sourcing.
func EnvBlob(vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		v := strings.ReplaceAll(vars[k], "'", `'\''`)
		fmt.Fprintf(&b, "%s='%s'\n", k, v)
	}
	return b.String()
}
