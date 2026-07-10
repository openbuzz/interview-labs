// Package hetzner wraps the read-only hcloud calls the CLI needs.
package hetzner

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

// ServerTypesFor lists non-deprecated server types priced in location,
// cheapest first, labeled with net EUR prices.
func ServerTypesFor(ctx context.Context, c *hcloud.Client,
	location string) ([]provider.Option, error) {
	types, err := c.ServerType.All(ctx)
	if err != nil {
		return nil, err
	}

	type row struct {
		opt    provider.Option
		hourly float64
	}
	var rows []row
	for _, st := range types {
		if st.IsDeprecated() {
			continue
		}
		hourly, monthly, ok := priceIn(st, location)
		if !ok {
			continue
		}
		rows = append(rows, row{
			hourly: hourly,
			opt: provider.Option{
				Slug: st.Name,
				Label: fmt.Sprintf("%s  %dvcpu %.0fGB %dGB  €%.3f/hr (€%.0f/mo)",
					st.Name, st.Cores, st.Memory, st.Disk, hourly, monthly),
			},
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].hourly < rows[j].hourly })

	out := make([]provider.Option, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.opt)
	}
	return out, nil
}

// priceIn extracts the net hourly/monthly EUR price of st in location.
func priceIn(st *hcloud.ServerType, location string) (float64, float64, bool) {
	for _, p := range st.Pricings {
		if p.Location == nil || p.Location.Name != location {
			continue
		}
		h, errH := strconv.ParseFloat(p.Hourly.Net, 64)
		m, errM := strconv.ParseFloat(p.Monthly.Net, 64)
		if errH != nil || errM != nil {
			return 0, 0, false
		}
		return h, m, true
	}
	return 0, 0, false
}
