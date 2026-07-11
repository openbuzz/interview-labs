package aws

import (
	"context"
	"os"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

// creds resolves the IAM user credential pair: a complete env pair beats
// the file pair; halves never mix.
func creds(cfg config.Config) (string, string) {
	id, secret := os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY")
	if id != "" && secret != "" {
		return id, secret
	}
	return cfg.Providers.AWS.AccessKeyID, cfg.Providers.AWS.SecretAccessKey
}

func (aw) Image() string   { return amiNameFilter }
func (aw) SSHUser() string { return "ubuntu" }

func (aw) EnvCreds(cfg config.Config) map[string]string {
	id, secret := creds(cfg)
	return map[string]string{
		"AWS_ACCESS_KEY_ID":     id,
		"AWS_SECRET_ACCESS_KEY": secret,
	}
}

func (aw) ValidateCreds(ctx context.Context, cfg config.Config) error {
	id, secret := creds(cfg)
	return ValidateCreds(ctx, NewSTS(id, secret))
}

func (aw) Regions(ctx context.Context, cfg config.Config) ([]provider.Option, error) {
	return Regions(), nil
}

func (aw) Sizes(ctx context.Context, cfg config.Config,
	region string) ([]provider.SizeInfo, error) {
	return InstanceTypes(), nil
}

func (aw) Defaults(cfg config.Config) (string, string) {
	return cfg.Providers.AWS.Region, cfg.Providers.AWS.Instance
}

func (aw) SetDefaults(cfg *config.Config, region, size string) {
	cfg.Providers.AWS.Region = region
	cfg.Providers.AWS.Instance = size
}
