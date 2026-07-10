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

func TestSizesForRegionSorted(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"sizes":[
		  {"slug":"s-2vcpu-4gb","available":true,"regions":["fra1"],
		   "price_monthly":24.0,"price_hourly":0.02976,"memory":4096,"vcpus":2,"disk":80},
		  {"slug":"s-1vcpu-1gb","available":true,"regions":["fra1","nyc1"],
		   "price_monthly":6.0,"price_hourly":0.00744,"memory":1024,"vcpus":1,"disk":25},
		  {"slug":"s-1vcpu-2gb","available":true,"regions":["nyc1"],
		   "price_monthly":12.0,"memory":2048,"vcpus":1,"disk":50},
		  {"slug":"gone","available":false,"regions":["fra1"],
		   "price_monthly":1.0,"memory":512,"vcpus":1,"disk":10}
		]}`))
	}))
	got, err := SizesFor(context.Background(), c, "fra1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "s-1vcpu-1gb" || got[1].Slug != "s-2vcpu-4gb" {
		t.Fatalf("SizesFor() = %+v", got)
	}
	if got[0].PriceHourly != 0.00744 {
		t.Fatalf("PriceHourly = %v, want 0.00744", got[0].PriceHourly)
	}
}

func TestImageConstant(t *testing.T) {
	if Image != "ubuntu-26-04-x64" {
		t.Fatalf("Image = %q", Image)
	}
}
