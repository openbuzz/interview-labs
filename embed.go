// Package interviewlabs carries assets embedded into the interview binary.
package interviewlabs

import "embed"

// InfraFS is the embedded terraform tree, staged per session at launch.
//
//go:embed all:terraform
var InfraFS embed.FS
