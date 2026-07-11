package hetzner

import (
	"context"
	"os"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

// token resolves the Hetzner Cloud token: env beats file.
func token(cfg config.Config) string {
	if t := os.Getenv("HCLOUD_TOKEN"); t != "" {
		return t
	}
	return cfg.Providers.Hetzner.Token
}

func (hz) Image() string   { return Image }
func (hz) SSHUser() string { return "root" }

func (hz) EnvCreds(cfg config.Config) map[string]string {
	return map[string]string{"HCLOUD_TOKEN": token(cfg)}
}

func (hz) ValidateCreds(ctx context.Context, cfg config.Config) error {
	return ValidateToken(ctx, NewClient(token(cfg)))
}

func (hz) Regions(ctx context.Context, cfg config.Config) ([]provider.Option, error) {
	return Locations(ctx, NewClient(token(cfg)))
}

func (hz) Sizes(ctx context.Context, cfg config.Config,
	region string) ([]provider.SizeInfo, error) {
	return ServerTypesFor(ctx, NewClient(token(cfg)), region)
}

func (hz) Defaults(cfg config.Config) (string, string) {
	return cfg.Providers.Hetzner.Region, cfg.Providers.Hetzner.Instance
}

func (hz) SetDefaults(cfg *config.Config, region, size string) {
	cfg.Providers.Hetzner.Region = region
	cfg.Providers.Hetzner.Instance = size
}
