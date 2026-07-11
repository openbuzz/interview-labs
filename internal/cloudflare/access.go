package cloudflare

import (
	"context"
	"os"

	"github.com/openbuzz/interview-labs/internal/config"
)

// token resolves the Cloudflare API token: env beats file.
func token(cfg config.Config) string {
	if t := os.Getenv("CLOUDFLARE_API_TOKEN"); t != "" {
		return t
	}
	return cfg.Providers.Cloudflare.APIToken
}

func (cf) ValidateCreds(ctx context.Context, cfg config.Config) error {
	return validateToken(ctx, token(cfg))
}

// EnvCreds returns the terraform child-process credential env; the
// cloudflare terraform provider reads CLOUDFLARE_API_TOKEN natively.
func (cf) EnvCreds(cfg config.Config) map[string]string {
	return map[string]string{"CLOUDFLARE_API_TOKEN": token(cfg)}
}

// TFVars are the root-module variables that activate the DNS module.
func (cf) TFVars(cfg config.Config) map[string]any {
	return map[string]any{
		"dns_enabled":        true,
		"cloudflare_zone_id": cfg.Providers.Cloudflare.ZoneID,
		"dns_base_domain":    cfg.Providers.Cloudflare.Domain,
	}
}
