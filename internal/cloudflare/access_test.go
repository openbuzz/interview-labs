package cloudflare

import (
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

var _ provider.Access = cf{}

func TestIdentityAndRoles(t *testing.T) {
	p := New()
	if p.Name() != "cloudflare" || p.Label() != "Cloudflare" {
		t.Fatalf("identity = %q/%q", p.Name(), p.Label())
	}
	if roles := p.Roles(); len(roles) != 1 || roles[0] != provider.RoleAccess {
		t.Fatalf("roles = %v", roles)
	}
}

func TestConfiguredNeedsTokenAndZone(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	var cfg config.Config
	if New().Configured(cfg) {
		t.Fatal("empty config reported configured")
	}

	cfg.Providers.Cloudflare.APIToken = "tok"
	if New().Configured(cfg) {
		t.Fatal("token without zone reported configured")
	}
	cfg.Providers.Cloudflare.ZoneID = "z1"
	if !New().Configured(cfg) {
		t.Fatal("token + zone not reported configured")
	}

	// env token + config zone is a valid mix
	cfg.Providers.Cloudflare.APIToken = ""
	t.Setenv("CLOUDFLARE_API_TOKEN", "env-tok")
	if !New().Configured(cfg) {
		t.Fatal("env token + config zone not reported configured")
	}
}

func TestEnvCredsAndTFVars(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	var cfg config.Config
	cfg.Providers.Cloudflare = config.Cloudflare{
		APIToken: "tok", ZoneID: "z1", Domain: "example.test",
	}

	creds := cf{}.EnvCreds(cfg)
	if creds["CLOUDFLARE_API_TOKEN"] != "tok" || len(creds) != 1 {
		t.Fatalf("creds = %v", creds)
	}

	vars := cf{}.TFVars(cfg)
	want := map[string]any{
		"dns_enabled":        true,
		"cloudflare_zone_id": "z1",
		"dns_base_domain":    "example.test",
	}
	if len(vars) != len(want) {
		t.Fatalf("vars = %v", vars)
	}
	for k, v := range want {
		if vars[k] != v {
			t.Fatalf("vars[%s] = %v, want %v", k, vars[k], v)
		}
	}
}
