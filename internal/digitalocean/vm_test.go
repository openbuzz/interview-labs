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

func TestSizeLabelShowsHourly(t *testing.T) {
	got := sizeLabel(Size{
		Slug: "s-1vcpu-1gb", VCPUs: 1, Memory: 1024, Disk: 25,
		PriceHourly: 0.00744, PriceMonthly: 5,
	})
	want := "s-1vcpu-1gb  1vcpu 1024MB 25GB  $0.007/hr ($5/mo)"
	if got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}
}
