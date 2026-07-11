package digitalocean

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/digitalocean/godo"
)

func testClient(t *testing.T, handler http.Handler) *godo.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient("test-token", godo.SetBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestValidateTokenOK(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"account":{"status":"active","email":"x@y.z"}}`))
	}))
	if err := ValidateToken(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTokenRejected(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"id":"unauthorized","message":"Unable to authenticate you"}`))
	}))
	err := ValidateToken(context.Background(), c)
	if err == nil {
		t.Fatal("ValidateToken accepted a 401")
	}
}

func TestRegionsFiltersUnavailable(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"regions":[
		  {"slug":"fra1","name":"Frankfurt 1","available":true},
		  {"slug":"nyc1","name":"New York 1","available":false}
		]}`))
	}))
	got, err := Regions(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Slug != "fra1" {
		t.Fatalf("Regions() = %+v", got)
	}
}

func TestSizesForFamilyAndMemoryFilter(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"sizes":[
		  {"slug":"s-2vcpu-4gb","available":true,"regions":["fra1"],"description":"Basic",
		   "price_hourly":0.036,"memory":4096,"vcpus":2,"disk":80},
		  {"slug":"s-2vcpu-4gb-amd","available":true,"regions":["fra1"],
		   "description":"Premium AMD","price_hourly":0.042,"memory":4096,"vcpus":2,"disk":80},
		  {"slug":"g-2vcpu-8gb","available":true,"regions":["fra1"],
		   "description":"General Purpose","price_hourly":0.0938,"memory":8192,"vcpus":2,
		   "disk":25},
		  {"slug":"gd-2vcpu-8gb","available":true,"regions":["fra1"],
		   "description":"General Purpose","price_hourly":0.1094,"memory":8192,"vcpus":2,
		   "disk":50},
		  {"slug":"s-1vcpu-2gb","available":true,"regions":["fra1"],"description":"Basic",
		   "price_hourly":0.018,"memory":2048,"vcpus":1,"disk":50},
		  {"slug":"s-32vcpu-128gb","available":true,"regions":["fra1"],"description":"Basic",
		   "price_hourly":1.143,"memory":131072,"vcpus":32,"disk":400},
		  {"slug":"c-4","available":true,"regions":["fra1"],"description":"CPU-Optimized",
		   "price_hourly":0.125,"memory":8192,"vcpus":4,"disk":50},
		  {"slug":"m-2vcpu-16gb","available":true,"regions":["fra1"],
		   "description":"Memory-Optimized","price_hourly":0.125,"memory":16384,"vcpus":2,
		   "disk":50},
		  {"slug":"s-4vcpu-8gb","available":true,"regions":["nyc1"],"description":"Basic",
		   "price_hourly":0.071,"memory":8192,"vcpus":4,"disk":160},
		  {"slug":"gone","available":false,"regions":["fra1"],"description":"Basic",
		   "price_hourly":0.01,"memory":4096,"vcpus":1,"disk":10}
		]}`))
	}))
	got, err := SizesFor(context.Background(), c, "fra1")
	if err != nil {
		t.Fatal(err)
	}

	// Survivors: s-/g-/gd- families, 4 <= mem GB <= 64, available, in region.
	want := []string{"s-2vcpu-4gb", "s-2vcpu-4gb-amd", "g-2vcpu-8gb", "gd-2vcpu-8gb"}
	if len(got) != len(want) {
		t.Fatalf("SizesFor() returned %d sizes: %+v", len(got), got)
	}
	for i, slug := range want {
		if got[i].Slug != slug {
			t.Fatalf("got[%d].Slug = %q, want %q", i, got[i].Slug, slug)
		}
	}
	if got[0].Description != "Basic" || got[2].Description != "General Purpose" {
		t.Fatalf("descriptions not carried: %+v", got)
	}
}

func TestSizesForMemoryBoundsInclusive(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"sizes":[
		  {"slug":"s-2vcpu-4gb","available":true,"regions":["fra1"],"description":"Basic",
		   "price_hourly":0.036,"memory":4096,"vcpus":2,"disk":80},
		  {"slug":"s-16vcpu-64gb","available":true,"regions":["fra1"],"description":"Basic",
		   "price_hourly":0.571,"memory":65536,"vcpus":16,"disk":800}
		]}`))
	}))
	got, err := SizesFor(context.Background(), c, "fra1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("4 GB and 64 GB rows must both survive: %+v", got)
	}
}

func TestImageConstant(t *testing.T) {
	if Image != "ubuntu-26-04-x64" {
		t.Fatalf("Image = %q", Image)
	}
}
