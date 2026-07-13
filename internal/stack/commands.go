package stack

import (
	"fmt"
	"sort"
	"strings"
)

// RemoteDir is where the stack lives on the VM, in RemoteDir/docker; the
// payload lands there too, alongside compose.yaml.
const RemoteDir = "/opt/interview"

// ComposeUpCmd starts the stack. Env arrives by sourcing stdin (EnvBlob),
// never argv — secrets stay out of /proc cmdlines. withOverride adds the
// engine-generated override file to the compose invocation.
func ComposeUpCmd(slug string, withOverride bool) string {
	files := ""
	if withOverride {
		files = "-f compose.yaml -f override.yaml "
	}
	return "cd " + RemoteDir + "/docker && set -a && . /dev/stdin && set +a && " +
		"docker compose " + files + "-p interview-" + slug + " up -d --wait"
}

// PushCmd receives one stack file arriving on stdin.
func PushCmd(name string) string {
	return "mkdir -p " + RemoteDir + "/docker && cat > " + RemoteDir +
		"/docker/" + name
}

// PushPayloadCmd extracts the payload tar arriving on stdin under
// RemoteDir/docker, alongside compose.yaml — the override's relative
// ./payload/... mounts resolve against the compose project dir.
func PushPayloadCmd() string {
	return "mkdir -p " + RemoteDir + "/docker && tar -xf - -C " + RemoteDir + "/docker"
}

// PullCmd pulls the session's two images. Refs cross as inline env (public
// registry names charset-validated at resolve time, not secrets) because
// compose interpolates GATEWAY_IMAGE/VSCODE_IMAGE at parse time — pull and
// up must see the same values.
func PullCmd(gatewayRef, vscodeRef string) string {
	return "cd " + RemoteDir + "/docker && GATEWAY_IMAGE='" + gatewayRef +
		"' VSCODE_IMAGE='" + vscodeRef + "' docker compose pull"
}

// SetupCmd runs the bundle's lab hook inside the vscode service. Values are
// charset-validated upstream (slugs, pack names); env crosses as exec -e so
// the hook sees exactly the documented variables and nothing else.
func SetupCmd(slug, bundle, scenarios string) string {
	return "cd " + RemoteDir + "/docker && docker compose " +
		"-f compose.yaml -f override.yaml -p interview-" + slug +
		" exec -T -e INTERVIEW_SESSION_ID=" + slug +
		" -e INTERVIEW_BUNDLE=" + bundle +
		" -e INTERVIEW_SCENARIOS=" + scenarios +
		" -e INTERVIEW_LAB_DIR=/opt/interview/lab" +
		" vscode bash /opt/interview/lab/setup.sh"
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
