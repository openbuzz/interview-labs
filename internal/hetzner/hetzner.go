// Package hetzner wraps the read-only hcloud calls the CLI needs.
package hetzner

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"

	"github.com/openbuzz/interview-labs/internal/provider"
)

// Image is the server image every session boots.
const Image = "ubuntu-26.04"

// NewClient builds an hcloud client (opts allow a test endpoint).
func NewClient(token string, opts ...hcloud.ClientOption) *hcloud.Client {
	return hcloud.NewClient(append([]hcloud.ClientOption{
		hcloud.WithToken(token),
	}, opts...)...)
}

// ValidateToken verifies the token with a read-only single-key list.
func ValidateToken(ctx context.Context, c *hcloud.Client) error {
	_, _, err := c.SSHKey.List(ctx, hcloud.SSHKeyListOpts{
		ListOpts: hcloud.ListOpts{PerPage: 1},
	})
	if err != nil {
		var herr hcloud.Error
		if errors.As(err, &herr) && herr.Code == hcloud.ErrorCodeUnauthorized {
			return fmt.Errorf("token rejected by Hetzner Cloud: %w", err)
		}
		return fmt.Errorf("could not validate token: %w", err)
	}
	return nil
}

// Locations lists datacenter locations as picker options.
func Locations(ctx context.Context, c *hcloud.Client) ([]provider.Option, error) {
	locs, err := c.Location.All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]provider.Option, 0, len(locs))
	for _, l := range locs {
		out = append(out, provider.Option{
			Slug:  l.Name,
			Label: fmt.Sprintf("%s  %s (%s)", l.Name, l.City, l.Country),
		})
	}
	return out, nil
}

// ServerTypesFor lists non-deprecated x86 shared and dedicated types priced
// in location, within the 4–64 GB memory window, in API order.
func ServerTypesFor(ctx context.Context, c *hcloud.Client,
	location string) ([]provider.SizeInfo, error) {
	types, err := c.ServerType.All(ctx)
	if err != nil {
		return nil, err
	}

	var out []provider.SizeInfo
	for _, st := range types {
		// amd64-only policy: deprecated and ARM (cax) types are not offered.
		if st.IsDeprecated() || st.Architecture != hcloud.ArchitectureX86 {
			continue
		}
		if st.Memory < 4 || st.Memory > 64 {
			continue
		}
		hourly, ok := priceIn(st, location)
		if !ok {
			continue
		}
		out = append(out, provider.SizeInfo{
			Slug:     st.Name,
			Category: category(st.CPUType),
			VCPUs:    st.Cores,
			MemGB:    int(math.Ceil(float64(st.Memory))),
			DiskGB:   st.Disk,
			Hourly:   hourly,
			Currency: "€",
		})
	}
	return out, nil
}

// category maps hcloud's CPU type to the picker's category column.
func category(t hcloud.CPUType) string {
	if t == hcloud.CPUTypeDedicated {
		return "Dedicated"
	}
	return "Shared"
}

// priceIn extracts the net hourly EUR price of st in location.
func priceIn(st *hcloud.ServerType, location string) (float64, bool) {
	for _, p := range st.Pricings {
		if p.Location == nil || p.Location.Name != location {
			continue
		}
		h, err := strconv.ParseFloat(p.Hourly.Net, 64)
		if err != nil {
			return 0, false
		}
		return h, true
	}
	return 0, false
}
