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
