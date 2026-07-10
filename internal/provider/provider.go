// Package provider is the capability seam between the CLI and cloud providers.
package provider

import (
	"context"

	"github.com/openbuzz/interview-labs/internal/config"
)

// Role is a capability a session needs from a provider.
type Role string

// Roles a provider can serve.
const (
	RoleVM     Role = "vm"
	RoleAI     Role = "ai"
	RoleAccess Role = "access"
)

// Provider is one configurable cloud provider. Regions, instance types and
// credential shapes stay concrete in each provider package.
type Provider interface {
	Name() string  // stable id: config key + terraform var value
	Label() string // display name
	Roles() []Role
	Configured(cfg config.Config) bool
	// Configure owns its TUI: guidance, prompts, validation, and writing its
	// block of cfg. The caller persists cfg afterwards.
	Configure(ctx context.Context, cfg *config.Config) error
}

// ByRole filters providers by capability, preserving order.
func ByRole(all []Provider, r Role) []Provider {
	var out []Provider
	for _, p := range all {
		for _, pr := range p.Roles() {
			if pr == r {
				out = append(out, p)
				break
			}
		}
	}
	return out
}
