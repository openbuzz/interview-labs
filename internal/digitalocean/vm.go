package digitalocean

import (
	"context"
	"os"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

// token resolves the DigitalOcean token: env beats file.
func token(cfg config.Config) string {
	if t := os.Getenv("DIGITALOCEAN_TOKEN"); t != "" {
		return t
	}
	return cfg.Providers.DigitalOcean.Token
}

func (do) Image() string   { return Image }
func (do) SSHUser() string { return "root" }

func (do) EnvCreds(cfg config.Config) map[string]string {
	return map[string]string{"DIGITALOCEAN_TOKEN": token(cfg)}
}

func (do) ValidateCreds(ctx context.Context, cfg config.Config) error {
	c, err := NewClient(token(cfg))
	if err != nil {
		return err
	}
	return ValidateToken(ctx, c)
}

func (do) Regions(ctx context.Context, cfg config.Config) ([]provider.Option, error) {
	c, err := NewClient(token(cfg))
	if err != nil {
		return nil, err
	}
	regions, err := Regions(ctx, c)
	if err != nil {
		return nil, err
	}

	out := make([]provider.Option, 0, len(regions))
	for _, r := range regions {
		out = append(out, provider.Option{Slug: r.Slug, Label: r.Slug + "  " + r.Name})
	}
	return out, nil
}

func (do) Sizes(ctx context.Context, cfg config.Config,
	region string) ([]provider.SizeInfo, error) {
	c, err := NewClient(token(cfg))
	if err != nil {
		return nil, err
	}
	sizes, err := SizesFor(ctx, c, region)
	if err != nil {
		return nil, err
	}

	out := make([]provider.SizeInfo, 0, len(sizes))
	for _, s := range sizes {
		out = append(out, toSizeInfo(s))
	}
	return out, nil
}

// toSizeInfo maps one droplet size onto the provider-neutral shape.
func toSizeInfo(s Size) provider.SizeInfo {
	return provider.SizeInfo{
		Slug:     s.Slug,
		Category: s.Description,
		VCPUs:    s.VCPUs,
		MemGB:    (s.Memory + 1023) / 1024,
		DiskGB:   s.Disk,
		Hourly:   s.PriceHourly,
		Currency: "$",
	}
}

func (do) Defaults(cfg config.Config) (string, string) {
	return cfg.Providers.DigitalOcean.Region, cfg.Providers.DigitalOcean.Instance
}

func (do) SetDefaults(cfg *config.Config, region, size string) {
	cfg.Providers.DigitalOcean.Region = region
	cfg.Providers.DigitalOcean.Instance = size
}
