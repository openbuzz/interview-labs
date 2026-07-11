package hetzner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"

	"github.com/openbuzz/interview-labs/internal/provider"
)

func testClient(t *testing.T, handler http.Handler) *hcloud.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient("test-token", hcloud.WithEndpoint(srv.URL))
}

func TestValidateTokenOK(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ssh_keys":[],"meta":{"pagination":{"page":1}}}`))
	}))
	if err := ValidateToken(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTokenRejected(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"unauthorized","message":"unable to authenticate"}}`))
	}))
	err := ValidateToken(context.Background(), c)
	if err == nil || !strings.Contains(err.Error(), "token rejected") {
		t.Fatalf("err = %v, want token rejected", err)
	}
}

func TestLocationsLabels(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"locations":[
		  {"id":1,"name":"fsn1","city":"Falkenstein","country":"DE"},
		  {"id":2,"name":"ash","city":"Ashburn, VA","country":"US"}
		],"meta":{"pagination":{"page":1}}}`))
	}))
	got, err := Locations(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "fsn1" ||
		got[0].Label != "fsn1  Falkenstein (DE)" {
		t.Fatalf("Locations() = %+v", got)
	}
}

// serverTypesFamilyFixture covers the family/memory filter boundaries: cx22
// and ccx13 are the plain surviving shared/dedicated pair; cx11 (deprecated)
// and cax11 (arm) are excluded upstream of the memory check; cx-tiny sits
// below the 4 GB floor, ccx63 above the 64 GB ceiling, and ccx53 sits right
// on that ceiling to prove it is inclusive.
const serverTypesFamilyFixture = `{"server_types":[
  {"id":2,"name":"cx22","cores":2,"memory":4.0,"disk":40,"deprecation":null,
   "architecture":"x86","cpu_type":"shared",
   "prices":[{"location":"fsn1",
     "price_hourly":{"net":"0.0060000000","gross":"0.0071"},
     "price_monthly":{"net":"3.9800000000","gross":"4.74"}}]},
  {"id":1,"name":"ccx13","cores":2,"memory":8.0,"disk":80,"deprecation":null,
   "architecture":"x86","cpu_type":"dedicated",
   "prices":[{"location":"fsn1",
     "price_hourly":{"net":"0.0230000000","gross":"0.0274"},
     "price_monthly":{"net":"14.2900000000","gross":"17.01"}}]},
  {"id":3,"name":"cx11","cores":1,"memory":2.0,"disk":20,
   "architecture":"x86","cpu_type":"shared",
   "deprecation":{"announced":"2024-01-01T00:00:00Z",
     "unavailable_after":"2024-06-01T00:00:00Z"},
   "prices":[{"location":"fsn1",
     "price_hourly":{"net":"0.0040000000","gross":"0.0048"},
     "price_monthly":{"net":"2.9900000000","gross":"3.56"}}]},
  {"id":4,"name":"cax11","cores":2,"memory":4.0,"disk":40,"deprecation":null,
   "architecture":"arm","cpu_type":"shared",
   "prices":[{"location":"fsn1",
     "price_hourly":{"net":"0.0050000000","gross":"0.0060"},
     "price_monthly":{"net":"3.2900000000","gross":"3.92"}}]},
  {"id":5,"name":"cx-tiny","cores":1,"memory":2.0,"disk":20,"deprecation":null,
   "architecture":"x86","cpu_type":"shared",
   "prices":[{"location":"fsn1",
     "price_hourly":{"net":"0.0030000000","gross":"0.0036"},
     "price_monthly":{"net":"1.9900000000","gross":"2.37"}}]},
  {"id":6,"name":"ccx63","cores":48,"memory":192.0,"disk":960,"deprecation":null,
   "architecture":"x86","cpu_type":"dedicated",
   "prices":[{"location":"fsn1",
     "price_hourly":{"net":"0.9950000000","gross":"1.1841"},
     "price_monthly":{"net":"640.0000000000","gross":"761.60"}}]},
  {"id":7,"name":"ccx53","cores":32,"memory":64.0,"disk":640,"deprecation":null,
   "architecture":"x86","cpu_type":"dedicated",
   "prices":[{"location":"fsn1",
     "price_hourly":{"net":"0.4270000000","gross":"0.5081"},
     "price_monthly":{"net":"274.0000000000","gross":"326.06"}}]}
],"meta":{"pagination":{"page":1}}}`

func TestServerTypesForFamilyAndMemoryFilter(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(serverTypesFamilyFixture))
	}))
	got, err := ServerTypesFor(context.Background(), c, "fsn1")
	if err != nil {
		t.Fatal(err)
	}

	// cx11 (deprecated), cax11 (arm), cx-tiny (<4GB) and ccx63 (>64GB) are
	// excluded; cx22, ccx13 and ccx53 survive, in API order.
	want := []provider.SizeInfo{
		{Slug: "cx22", Category: "Shared", VCPUs: 2, MemGB: 4, DiskGB: 40,
			Hourly: 0.006, Currency: "€"},
		{Slug: "ccx13", Category: "Dedicated", VCPUs: 2, MemGB: 8, DiskGB: 80,
			Hourly: 0.023, Currency: "€"},
		{Slug: "ccx53", Category: "Dedicated", VCPUs: 32, MemGB: 64, DiskGB: 640,
			Hourly: 0.427, Currency: "€"},
	}
	if len(got) != len(want) {
		t.Fatalf("ServerTypesFor() returned %d types: %+v", len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("got[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}
