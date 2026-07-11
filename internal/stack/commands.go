package stack

import (
	"fmt"
	"sort"
	"strings"
)

// RemoteDir is where the payload lands on the VM; the stack itself lives
// in RemoteDir/docker.
const RemoteDir = "/opt/interview"

// ExtractCmd unpacks the payload tar arriving on stdin.
func ExtractCmd() string {
	return "mkdir -p " + RemoteDir + "/docker && tar -xzf - -C " + RemoteDir + "/docker"
}

// BakeCmd builds the gateway and the selected vscode profile in one bake run.
func BakeCmd(profile string) string {
	return "cd " + RemoteDir + "/docker && docker buildx bake gateway " + profile
}

// ComposeUpCmd starts the stack. Env arrives by sourcing stdin (EnvBlob),
// never argv — secrets stay out of /proc cmdlines.
func ComposeUpCmd(slug string) string {
	return "cd " + RemoteDir + "/docker && set -a && . /dev/stdin && set +a && " +
		"docker compose -p interview-" + slug + " up -d --wait"
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
