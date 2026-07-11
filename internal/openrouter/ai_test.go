package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

var _ provider.AI = or{}

func TestIdentityAndRoles(t *testing.T) {
	p := New()
	if p.Name() != "openrouter" || p.Label() != "OpenRouter" {
		t.Fatalf("identity = %q/%q", p.Name(), p.Label())
	}
	if roles := p.Roles(); len(roles) != 1 || roles[0] != provider.RoleAI {
		t.Fatalf("roles = %v", roles)
	}
}

func TestConfiguredEnvBeatsConfig(t *testing.T) {
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	var cfg config.Config
	if New().Configured(cfg) {
		t.Fatal("empty config reported configured")
	}

	cfg.Providers.OpenRouter.ManagementKey = "file-key"
	if managementKey(cfg) != "file-key" {
		t.Fatal("config key not used")
	}
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "env-key")
	if managementKey(cfg) != "env-key" {
		t.Fatal("env must beat config")
	}
	if !New().Configured(config.Config{}) {
		t.Fatal("env key alone must report configured")
	}
}

func TestCapUSDDefaultsTo10(t *testing.T) {
	var cfg config.Config
	if got := capUSD(cfg); got != DefaultCapUSD {
		t.Fatalf("capUSD(zero) = %v, want %v", got, DefaultCapUSD)
	}
	cfg.Providers.OpenRouter.CapUSD = 2.5
	if got := capUSD(cfg); got != 2.5 {
		t.Fatalf("capUSD = %v", got)
	}
}

func TestMintRevokeThroughCapability(t *testing.T) {
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	var sawLabel string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				var body map[string]any
				_ = jsonDecode(r, &body)
				sawLabel, _ = body["name"].(string)
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"key":"sk","data":{"hash":"h9"}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
	defer srv.Close()
	swapBase(t, srv.URL)

	key, err := or{}.Mint(context.Background(), config.Config{}, "calm-otter")
	if err != nil {
		t.Fatal(err)
	}
	if key.Hash != "h9" || key.CapUSD != DefaultCapUSD {
		t.Fatalf("minted = %+v", key)
	}
	if sawLabel != "interview-labs-calm-otter" {
		t.Fatalf("label = %q", sawLabel)
	}
	if err := (or{}).Revoke(context.Background(), config.Config{}, "h9"); err != nil {
		t.Fatal(err)
	}
}

func jsonDecode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
