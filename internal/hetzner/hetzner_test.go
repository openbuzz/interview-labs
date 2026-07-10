package hetzner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
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

func TestServerTypesForLocationSortedCheapestFirst(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"server_types":[
		  {"id":1,"name":"ccx13","cores":2,"memory":8.0,"disk":80,"deprecation":null,
		   "prices":[{"location":"fsn1",
		     "price_hourly":{"net":"0.0230000000","gross":"0.0274"},
		     "price_monthly":{"net":"14.2900000000","gross":"17.01"}}]},
		  {"id":2,"name":"cx22","cores":2,"memory":4.0,"disk":40,"deprecation":null,
		   "prices":[{"location":"fsn1",
		     "price_hourly":{"net":"0.0060000000","gross":"0.0071"},
		     "price_monthly":{"net":"3.9800000000","gross":"4.74"}}]},
		  {"id":3,"name":"cx11","cores":1,"memory":2.0,"disk":20,
		   "deprecation":{"announced":"2024-01-01T00:00:00Z",
		     "unavailable_after":"2024-06-01T00:00:00Z"},
		   "prices":[{"location":"fsn1",
		     "price_hourly":{"net":"0.0040000000","gross":"0.0048"},
		     "price_monthly":{"net":"2.9900000000","gross":"3.56"}}]},
		  {"id":4,"name":"cax11","cores":2,"memory":4.0,"disk":40,"deprecation":null,
		   "prices":[{"location":"nbg1",
		     "price_hourly":{"net":"0.0050000000","gross":"0.0060"},
		     "price_monthly":{"net":"3.2900000000","gross":"3.92"}}]}
		],"meta":{"pagination":{"page":1}}}`))
	}))
	got, err := ServerTypesFor(context.Background(), c, "fsn1")
	if err != nil {
		t.Fatal(err)
	}

	// cx11 is deprecated, cax11 is not priced in fsn1 — both filtered out;
	// cx22 sorts before ccx13 (cheaper).
	if len(got) != 2 || got[0].Slug != "cx22" || got[1].Slug != "ccx13" {
		t.Fatalf("ServerTypesFor() = %+v", got)
	}
	want := "cx22  2vcpu 4GB 40GB  €0.006/hr (€4/mo)"
	if got[0].Label != want {
		t.Fatalf("label = %q, want %q", got[0].Label, want)
	}
}
