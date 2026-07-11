package cloudflare

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func swapBase(t *testing.T, url string) {
	t.Helper()
	old := baseURL
	baseURL = url
	t.Cleanup(func() { baseURL = old })
}

func TestValidateTokenRequiresActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/user/tokens/verify" {
				t.Errorf("path = %s", r.URL.Path)
			}
			switch r.Header.Get("Authorization") {
			case "Bearer active-tok":
				w.Write([]byte(`{"success":true,"result":{"status":"active"}}`))
			case "Bearer disabled-tok":
				w.Write([]byte(`{"success":true,"result":{"status":"disabled"}}`))
			default:
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"success":false,"errors":[{"message":"invalid token"}]}`))
			}
		}))
	defer srv.Close()
	swapBase(t, srv.URL)

	if err := validateToken(context.Background(), "active-tok"); err != nil {
		t.Fatalf("active token rejected: %v", err)
	}
	if err := validateToken(context.Background(), "disabled-tok"); err == nil ||
		!strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled token: err = %v", err)
	}
	if err := validateToken(context.Background(), "bad-tok"); err == nil ||
		!strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("rejected token: err = %v", err)
	}
}

func TestZonesListsActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/zones" {
				t.Errorf("path = %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("status") != "active" || q.Get("per_page") != "50" {
				t.Errorf("query = %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"success":true,"result":[` +
				`{"id":"z1","name":"example.test"},` +
				`{"id":"z2","name":"other.example"}]}`))
		}))
	defer srv.Close()
	swapBase(t, srv.URL)

	got, err := zones(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (Zone{ID: "z1", Name: "example.test"}) {
		t.Fatalf("zones = %+v", got)
	}
}

func TestZonesErrorSurfacesMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"success":false,"errors":[{"message":"needs Zone.Zone read"}]}`))
		}))
	defer srv.Close()
	swapBase(t, srv.URL)

	_, err := zones(context.Background(), "tok")
	if err == nil || !strings.Contains(err.Error(), "needs Zone.Zone read") {
		t.Fatalf("err = %v", err)
	}
}
