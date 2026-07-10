package aws

import (
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

var _ provider.VM = aw{}

func TestEnvCredsPairSemantics(t *testing.T) {
	var cfg config.Config
	cfg.Providers.AWS.AccessKeyID = "file-id"
	cfg.Providers.AWS.SecretAccessKey = "file-sec"

	// complete env pair wins
	t.Setenv("AWS_ACCESS_KEY_ID", "env-id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "env-sec")
	creds := (aw{}).EnvCreds(cfg)
	if creds["AWS_ACCESS_KEY_ID"] != "env-id" ||
		creds["AWS_SECRET_ACCESS_KEY"] != "env-sec" {
		t.Fatalf("env pair = %+v", creds)
	}

	// a half-set env pair never mixes with the file pair
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	creds = (aw{}).EnvCreds(cfg)
	if creds["AWS_ACCESS_KEY_ID"] != "file-id" ||
		creds["AWS_SECRET_ACCESS_KEY"] != "file-sec" {
		t.Fatalf("file pair = %+v", creds)
	}
}

func TestConfiguredNeedsBothHalves(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	var cfg config.Config
	if (aw{}).Configured(cfg) {
		t.Fatal("empty config reads configured")
	}
	cfg.Providers.AWS.AccessKeyID = "id"
	if (aw{}).Configured(cfg) {
		t.Fatal("half a credential reads configured")
	}
	cfg.Providers.AWS.SecretAccessKey = "sec"
	if !(aw{}).Configured(cfg) {
		t.Fatal("full credential reads unconfigured")
	}
}

func TestIdentityAndStatics(t *testing.T) {
	p := New()
	if p.Name() != "aws" || p.Label() != "AWS" {
		t.Fatalf("identity = %s/%s", p.Name(), p.Label())
	}
	if got := (aw{}).SSHUser(); got != "ubuntu" {
		t.Fatalf("ssh user = %q", got)
	}
	if got := (aw{}).Image(); got != amiNameFilter {
		t.Fatalf("image = %q", got)
	}
}

func TestRegionsCurated(t *testing.T) {
	var cfg config.Config
	got, err := (aw{}).Regions(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0].Slug != "eu-central-1" {
		t.Fatalf("regions = %+v", got)
	}
}
