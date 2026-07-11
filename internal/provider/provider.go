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

// Option is one pickable region: Slug is the value handed to terraform,
// Label the display row.
type Option struct {
	Slug  string
	Label string
}

// SizeInfo is one pickable instance size; the CLI owns sort and rendering.
type SizeInfo struct {
	Slug     string
	Category string // DO: API description; Hetzner: Shared|Dedicated; AWS: General Purpose
	VCPUs    int
	MemGB    int // provider memory rounded up to whole GB
	DiskGB   int
	Hourly   float64 // provider-native currency, net where the API distinguishes
	Currency string  // "$" or "€"
}

// VM is the capability interface for providers that can host a session VM.
// Launch, doctor and destroy drive it; nothing outside a provider package
// special-cases a provider name.
type VM interface {
	Provider
	// Image is the OS image value passed to terraform: an image slug, or
	// for AWS the AMI name filter its module resolves.
	Image() string
	// SSHUser is the login user on a fresh VM.
	SSHUser() string
	// EnvCreds returns the terraform child-process credential env.
	EnvCreds(cfg config.Config) map[string]string
	// ValidateCreds performs one cheap authenticated read.
	ValidateCreds(ctx context.Context, cfg config.Config) error
	Regions(ctx context.Context, cfg config.Config) ([]Option, error)
	Sizes(ctx context.Context, cfg config.Config, region string) ([]SizeInfo, error)
	// Defaults returns the persisted launch preselects.
	Defaults(cfg config.Config) (region, size string)
	// SetDefaults records the operator's picks; the caller persists cfg.
	SetDefaults(cfg *config.Config, region, size string)
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
