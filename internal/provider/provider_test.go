package provider

import (
	"context"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
)

type fake struct {
	name  string
	roles []Role
}

func (f fake) Name() string                  { return f.name }
func (f fake) Label() string                 { return f.name }
func (f fake) Roles() []Role                 { return f.roles }
func (f fake) Configured(config.Config) bool { return true }

func (f fake) Configure(context.Context, *config.Config) error { return nil }

func TestByRole(t *testing.T) {
	vm := fake{name: "vm-only", roles: []Role{RoleVM}}
	both := fake{name: "vm-and-access", roles: []Role{RoleVM, RoleAccess}}
	ai := fake{name: "ai-only", roles: []Role{RoleAI}}
	all := []Provider{vm, both, ai}

	got := ByRole(all, RoleVM)
	if len(got) != 2 || got[0].Name() != "vm-only" || got[1].Name() != "vm-and-access" {
		t.Fatalf("ByRole(vm) = %v", got)
	}
	if got := ByRole(all, RoleAccess); len(got) != 1 || got[0].Name() != "vm-and-access" {
		t.Fatalf("ByRole(access) = %v", got)
	}
	if got := ByRole(nil, RoleVM); got != nil {
		t.Fatalf("ByRole(nil) = %v, want nil", got)
	}
}

// fakeVM proves the VM contract is implementable and keeps ByRole working
// on values that carry the capability.
type fakeVM struct{}

func (fakeVM) Name() string                                       { return "fake" }
func (fakeVM) Label() string                                      { return "Fake" }
func (fakeVM) Roles() []Role                                      { return []Role{RoleVM} }
func (fakeVM) Configured(config.Config) bool                      { return true }
func (fakeVM) Configure(context.Context, *config.Config) error    { return nil }
func (fakeVM) Image() string                                      { return "img" }
func (fakeVM) SSHUser() string                                    { return "root" }
func (fakeVM) EnvCreds(config.Config) map[string]string           { return nil }
func (fakeVM) ValidateCreds(context.Context, config.Config) error { return nil }
func (fakeVM) Regions(context.Context, config.Config) ([]Option, error) {
	return []Option{{Slug: "r1", Label: "r1  Region"}}, nil
}
func (fakeVM) Sizes(context.Context, config.Config, string) ([]SizeInfo, error) {
	return nil, nil
}
func (fakeVM) Defaults(config.Config) (string, string)    { return "", "" }
func (fakeVM) SetDefaults(*config.Config, string, string) {}

func TestVMSatisfiesProvider(t *testing.T) {
	var vm VM = fakeVM{}

	got := ByRole([]Provider{vm}, RoleVM)
	if len(got) != 1 {
		t.Fatalf("ByRole kept %d providers, want 1", len(got))
	}
	if _, ok := got[0].(VM); !ok {
		t.Fatal("VM capability lost through ByRole")
	}
}
