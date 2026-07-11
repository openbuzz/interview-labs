// Package digitalocean wraps the read-only godo calls the CLI needs.
package digitalocean

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

// Image is the droplet image every session boots.
const Image = "ubuntu-26-04-x64"

// NewClient builds a godo client (opts allow a test base URL).
func NewClient(token string, opts ...godo.ClientOpt) (*godo.Client, error) {
	if len(opts) == 0 {
		return godo.NewFromToken(token), nil
	}
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return godo.New(oauth2.NewClient(context.Background(), src), opts...)
}

// ValidateToken verifies the token with a read-only account fetch.
func ValidateToken(ctx context.Context, c *godo.Client) error {
	_, resp, err := c.Account.Get(ctx)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("token rejected by DigitalOcean: %w", err)
		}
		return fmt.Errorf("could not validate token: %w", err)
	}
	return nil
}

// Region is a selectable droplet region.
type Region struct {
	Slug string
	Name string
}

// Size is a selectable droplet size.
type Size struct {
	Slug         string
	Description  string
	PriceHourly  float64
	PriceMonthly float64
	Memory       int
	VCPUs        int
	Disk         int
}

// Regions lists available regions.
func Regions(ctx context.Context, c *godo.Client) ([]Region, error) {
	all, _, err := c.Regions.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return nil, err
	}

	var out []Region
	for _, r := range all {
		if r.Available {
			out = append(out, Region{Slug: r.Slug, Name: r.Name})
		}
	}
	return out, nil
}

// SizesFor lists sizes offered in region: shared and general-purpose
// families (s-, g-, gd-) within the 4–64 GB memory window, in API order.
func SizesFor(ctx context.Context, c *godo.Client, region string) ([]Size, error) {
	all, _, err := c.Sizes.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return nil, err
	}

	var out []Size
	for _, s := range all {
		if !s.Available || !contains(s.Regions, region) {
			continue
		}
		if !generalPurpose(s.Slug) || s.Memory < 4096 || s.Memory > 65536 {
			continue
		}
		out = append(out, Size{
			Slug:         s.Slug,
			Description:  s.Description,
			PriceHourly:  s.PriceHourly,
			PriceMonthly: s.PriceMonthly,
			Memory:       s.Memory,
			VCPUs:        s.Vcpus,
			Disk:         s.Disk,
		})
	}
	return out, nil
}

// generalPurpose reports whether slug is a shared (s-) or general-purpose
// dedicated (g-, gd-) droplet family.
func generalPurpose(slug string) bool {
	return strings.HasPrefix(slug, "s-") || strings.HasPrefix(slug, "g-") ||
		strings.HasPrefix(slug, "gd-")
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
