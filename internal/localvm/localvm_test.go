package localvm

import (
	"context"
	"errors"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

func TestIdentityAndRoles(t *testing.T) {
	p := New()
	if p.Name() != "local" || p.Label() != "Local Docker" {
		t.Fatalf("identity = %s/%s", p.Name(), p.Label())
	}
	if len(p.Roles()) != 1 || p.Roles()[0] != provider.RoleVM {
		t.Fatalf("roles = %v", p.Roles())
	}
	if _, isVM := p.(provider.VM); isVM {
		t.Fatal("local must NOT satisfy provider.VM — launch dispatches on that")
	}
}

func TestConfiguredTracksDockerCLI(t *testing.T) {
	old := lookDocker
	t.Cleanup(func() { lookDocker = old })

	lookDocker = func() error { return nil }
	if !New().Configured(config.Config{}) {
		t.Fatal("docker present ⇒ configured")
	}
	lookDocker = func() error { return errors.New("nope") }
	if New().Configured(config.Config{}) {
		t.Fatal("docker absent ⇒ not configured")
	}
}

func TestValidateCredsProbesDaemon(t *testing.T) {
	oldLook, oldInfo := lookDocker, dockerInfo
	t.Cleanup(func() { lookDocker, dockerInfo = oldLook, oldInfo })
	lookDocker = func() error { return nil }
	dockerInfo = func(context.Context) error { return errors.New("daemon down") }

	v, ok := New().(provider.CredentialValidator)
	if !ok {
		t.Fatal("local must satisfy CredentialValidator for doctor")
	}
	if err := v.ValidateCreds(context.Background(), config.Config{}); err == nil {
		t.Fatal("dead daemon must fail validation")
	}
}
