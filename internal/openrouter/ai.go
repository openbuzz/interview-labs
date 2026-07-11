package openrouter

import (
	"context"
	"os"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

// DefaultCapUSD is the per-session spend cap when config sets none.
const DefaultCapUSD float64 = 10

// managementKey resolves the operator credential: env beats file.
func managementKey(cfg config.Config) string {
	if k := os.Getenv("OPENROUTER_MANAGEMENT_KEY"); k != "" {
		return k
	}
	return cfg.Providers.OpenRouter.ManagementKey
}

// capUSD resolves the per-session spend cap; 0 in config means the default.
func capUSD(cfg config.Config) float64 {
	if c := cfg.Providers.OpenRouter.CapUSD; c > 0 {
		return c
	}
	return DefaultCapUSD
}

func (or) ValidateCreds(ctx context.Context, cfg config.Config) error {
	return validate(ctx, managementKey(cfg))
}

// Mint creates the session's spend-capped API key; the key value is
// discarded by the client, only the revoke handle and cap survive.
func (or) Mint(ctx context.Context, cfg config.Config,
	slug string) (provider.MintedKey, error) {
	cap := capUSD(cfg)
	hash, err := mint(ctx, managementKey(cfg), cap, "interview-labs-"+slug)
	if err != nil {
		return provider.MintedKey{}, err
	}
	return provider.MintedKey{Hash: hash, CapUSD: cap}, nil
}

func (or) Revoke(ctx context.Context, cfg config.Config, hash string) error {
	return revoke(ctx, managementKey(cfg), hash)
}
