// Package interviewlabs carries assets embedded into the interview binary.
package interviewlabs

import "embed"

// InfraFS is the embedded terraform tree, staged per session at launch.
// Patterns name the sources explicitly so a stray .terraform/ from an
// in-repo init never rides along (the lock file is the one dot-path wanted).
//
//go:embed terraform/*.tf terraform/.terraform.lock.hcl
//go:embed terraform/digitalocean terraform/hetzner terraform/aws terraform/cloudflare
var InfraFS embed.FS

// DockerFS is the embedded container-stack tree — compose, bake and the
// vscode layers — staged per session and pushed to the VM as a tar payload.
// Patterns name the sources explicitly: no tests, no task cache (a
// directory pattern would embed whatever a dirty working tree contains).
//
//go:embed docker/compose.yaml docker/docker-bake.hcl
//go:embed docker/images/vscode/layers
var DockerFS embed.FS

// GatewayFS is the gateway build context. go:embed cannot cross into
// docker/images/gateway (its go.mod makes it a separate module), so a
// filtered verbatim copy of its build inputs lives under _payload/ — a
// _-prefixed dir the toolchain ignores for package loading. The embed walk
// also skips any subdir holding a go.mod, so the copy parks it as
// go.mod.payload and Stage restores the name while merging the tree back
// to images/gateway/. Regenerate with `task docker:payload`;
// TestPayloadInSync guards drift.
//
//go:embed all:_payload
var GatewayFS embed.FS
