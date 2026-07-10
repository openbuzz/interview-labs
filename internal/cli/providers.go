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
