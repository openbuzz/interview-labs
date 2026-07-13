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

// DockerFS is the embedded compose file — the one artifact a session stages;
// images are prebuilt in CI and pulled by ref.
//
//go:embed docker/compose.yaml
var DockerFS embed.FS

// SpecFS carries the pack contract schemas; pack.Load compiles them from
// here so validation works offline — the https $id is a label, not a fetch.
//
//go:embed spec/pack/v1
var SpecFS embed.FS

// PacksFS carries the embedded content packs: default (real interview
// content) and template (the pack-init scaffold source).
//
// A scenario task/ tree that is itself a Go module cannot be named go.mod
// on disk: cmd/go treats any directory containing that exact filename as a
// separate module and silently drops its whole subtree from the embed, no
// build error. Those files are checked in as _go.mod (still picked up by
// all:) — a future staging step must rename _go.mod back to go.mod when
// materializing a scenario's task/ tree to disk for the candidate.
//
//go:embed all:packs
var PacksFS embed.FS
