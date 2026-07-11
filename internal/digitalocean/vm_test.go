package digitalocean

import (
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

var _ provider.VM = do{}

func TestEnvCredsEnvBeatsFile(t *testing.T) {
	var cfg config.Config
	cfg.Providers.DigitalOcean.Token = "file-tok"

	t.Setenv("DIGITALOCEAN_TOKEN", "env-tok")
	if got := (do{}).EnvCreds(cfg)["DIGITALOCEAN_TOKEN"]; got != "env-tok" {
		t.Fatalf("env creds = %q, want env-tok", got)
	}

	t.Setenv("DIGITALOCEAN_TOKEN", "")
	if got := (do{}).EnvCreds(cfg)["DIGITALOCEAN_TOKEN"]; got != "file-tok" {
		t.Fatalf("file creds = %q, want file-tok", got)
	}
}

func TestVMStaticValues(t *testing.T) {
	if got := (do{}).Image(); got != "ubuntu-26-04-x64" {
		t.Fatalf("image = %q", got)
	}
	if got := (do{}).SSHUser(); got != "root" {
		t.Fatalf("ssh user = %q", got)
	}
}

func TestDefaultsRoundTrip(t *testing.T) {
	var cfg config.Config
	(do{}).SetDefaults(&cfg, "fra1", "s-2vcpu-2gb")

	r, s := (do{}).Defaults(cfg)
	if r != "fra1" || s != "s-2vcpu-2gb" {
		t.Fatalf("defaults = %q/%q", r, s)
	}
	if cfg.Providers.DigitalOcean.Region != "fra1" {
		t.Fatalf("cfg region = %q", cfg.Providers.DigitalOcean.Region)
	}
}

func TestToSizeInfo(t *testing.T) {
	got := toSizeInfo(Size{
		Slug: "s-2vcpu-4gb", Description: "Basic", VCPUs: 2, Memory: 4096, Disk: 80,
		PriceHourly: 0.036,
	})
	want := provider.SizeInfo{
		Slug: "s-2vcpu-4gb", Category: "Basic", VCPUs: 2, MemGB: 4, DiskGB: 80,
		Hourly: 0.036, Currency: "$",
	}
	if got != want {
		t.Fatalf("toSizeInfo() = %+v, want %+v", got, want)
	}

	if got := toSizeInfo(Size{Memory: 4097}).MemGB; got != 5 {
		t.Fatalf("MemGB round-up for 4097 MB = %d, want 5", got)
	}
}
