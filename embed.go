// Package interviewlabs carries assets embedded into the interview binary.
package interviewlabs

import "embed"

// InfraFS is the embedded terraform tree, staged per session at launch.
// Patterns name the sources explicitly so a stray .terraform/ from an
// in-repo init never rides along (the lock file is the one dot-path wanted).
//
//go:embed terraform/*.tf terraform/.terraform.lock.hcl terraform/digitalocean
var InfraFS embed.FS
