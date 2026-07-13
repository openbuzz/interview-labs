// Package pack loads and validates content packs: the org/department unit
// whose bundles each describe one interview position.
package pack

import "io/fs"

// Pack is one validated content pack.
type Pack struct {
	Contract    int
	Name        string
	Version     string
	Description string
	Bundles     []Bundle
	FS          fs.FS
}

// Bundle is one position: its scenarios plus optional lab hook and kind
// cluster.
type Bundle struct {
	Name        string
	Description string
	Image       string
	HasLab      bool
	HasSetup    bool
	HasKind     bool
	Scenarios   []Scenario
}

// Scenario is one task given to the candidate.
type Scenario struct {
	Name        string
	Title       string
	Description string
}

// BundleByName finds a bundle in p.
func BundleByName(p *Pack, name string) (*Bundle, bool) {
	for i := range p.Bundles {
		if p.Bundles[i].Name == name {
			return &p.Bundles[i], true
		}
	}
	return nil, false
}
