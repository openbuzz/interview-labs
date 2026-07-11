package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// swapBase points the client at a test server for the test's duration.
func swapBase(t *testing.T, url string) {
	t.Helper()
	old := BaseURL
	BaseURL = url
	t.Cleanup(func() { BaseURL = old })
}

func TestValidateAccepts2xxRejectsAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/keys" || r.Method != http.MethodGet {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer good" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":{"message":"bad key"}}`))
				return
			}
			w.Write([]byte(`{"data":[]}`))
		}))
	defer srv.Close()
	swapBase(t, srv.URL)

	if err := validate(context.Background(), "good"); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}

	err := validate(context.Background(), "bad")
	if err == nil {
		t.Fatal("invalid key accepted")
	}
	if !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("provider message lost: %v", err)
	}
	if strings.Contains(err.Error(), "bad") && strings.Contains(err.Error(), "Bearer") {
		t.Fatalf("error leaks auth header: %v", err)
	}
}

func TestMintReturnsHashDiscardsKey(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/keys" {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Error(err)
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"key":"sk-or-child","data":{"hash":"hash-1"}}`))
		}))
	defer srv.Close()
	swapBase(t, srv.URL)

	hash, err := mint(context.Background(), "mk", 10, "interview-labs-calm-otter")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "hash-1" {
		t.Fatalf("hash = %q", hash)
	}
	if gotBody["name"] != "interview-labs-calm-otter" || gotBody["limit"] != float64(10) {
		t.Fatalf("mint body = %v", gotBody)
	}
}

func TestMintMissingHashErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"key":"sk-or-child","data":{}}`))
		}))
	defer srv.Close()
	swapBase(t, srv.URL)

	if _, err := mint(context.Background(), "mk", 10, "x"); err == nil {
		t.Fatal("hash-less response accepted")
	}
}

func TestRevoke404IsSuccess(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.Method+" "+r.URL.Path)
			switch len(paths) {
			case 1:
				w.WriteHeader(http.StatusOK)
			case 2:
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":{"message":"not found"}}`))
			default:
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":{"message":"boom"}}`))
			}
		}))
	defer srv.Close()
	swapBase(t, srv.URL)

	if err := revoke(context.Background(), "mk", "hash-1"); err != nil {
		t.Fatalf("2xx revoke: %v", err)
	}
	if err := revoke(context.Background(), "mk", "hash-1"); err != nil {
		t.Fatalf("404 revoke must be success (idempotent rerun): %v", err)
	}
	if err := revoke(context.Background(), "mk", "hash-1"); err == nil {
		t.Fatal("500 revoke accepted")
	}
	if paths[0] != "DELETE /keys/hash-1" {
		t.Fatalf("revoke path = %q", paths[0])
	}
}
