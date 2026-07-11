package cli

import "github.com/openbuzz/interview-labs/internal/provider"

// vmByName finds a registered VM-capable provider by its stable name.
func vmByName(name string) (provider.VM, bool) {
	for _, p := range provider.ByRole(providers, provider.RoleVM) {
		if vm, ok := p.(provider.VM); ok && p.Name() == name {
			return vm, true
		}
	}
	return nil, false
}

// aiByName finds a registered AI-capable provider by its stable name.
func aiByName(name string) (provider.AI, bool) {
	for _, p := range provider.ByRole(providers, provider.RoleAI) {
		if ai, ok := p.(provider.AI); ok && p.Name() == name {
			return ai, true
		}
	}
	return nil, false
}

// accessByName finds a registered access-capable provider by its stable name.
func accessByName(name string) (provider.Access, bool) {
	for _, p := range provider.ByRole(providers, provider.RoleAccess) {
		if acc, ok := p.(provider.Access); ok && p.Name() == name {
			return acc, true
		}
	}
	return nil, false
}
