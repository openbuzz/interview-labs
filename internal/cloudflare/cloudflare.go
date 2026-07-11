// Package cloudflare is the Cloudflare access provider: one proxied DNS
// record per session, created and destroyed by the session's terraform.
// Plain net/http for the two init-time calls (verify, zone list) — the
// heavy lifting lives in the terraform provider, not here.
package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// baseURL is Cloudflare API v4's base URL; package var so tests point it at
// an httptest server. Unexported: unlike openrouter.BaseURL, no test outside
// this package ever calls the Cloudflare API.
var baseURL = "https://api.cloudflare.com/client/v4"

// httpClient issues every request; calls are bounded by ctx.
var httpClient = http.DefaultClient

// envelope is Cloudflare v4's response wrapper.
type envelope struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Zone is one pickable DNS zone.
type Zone struct {
	ID   string
	Name string
}

// validateToken checks token against Cloudflare's token-verify endpoint:
// valid only when the call succeeds AND the token's status is "active" (a
// revoked-but-not-deleted token can still authenticate the verify call).
// Errors never include the token.
func validateToken(ctx context.Context, token string) error {
	var out struct {
		envelope
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := get(ctx, "/user/tokens/verify", token, &out); err != nil {
		return fmt.Errorf("cloudflare: validate token: %w", err)
	}
	if !out.Success || out.Result.Status != "active" {
		return fmt.Errorf("cloudflare: validate token: token status: %s", out.Result.Status)
	}
	return nil
}

// zones lists the account's active zones for the init-time picker.
func zones(ctx context.Context, token string) ([]Zone, error) {
	var out struct {
		envelope
		Result []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := get(ctx, "/zones?status=active&per_page=50", token, &out); err != nil {
		return nil, fmt.Errorf("cloudflare: list zones: %w", err)
	}

	list := make([]Zone, 0, len(out.Result))
	for _, z := range out.Result {
		list = append(list, Zone{ID: z.ID, Name: z.Name})
	}
	return list, nil
}

// get runs one Bearer-authenticated GET under baseURL, decoding the JSON
// response into out. Non-2xx errors carry Cloudflare's own message, never
// the raw body or the token.
func get(ctx context.Context, path, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	decodeErr := json.Unmarshal(data, out)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, apiMessage(data))
	}
	if decodeErr != nil {
		return fmt.Errorf("decode response: %w", decodeErr)
	}
	return nil
}

// apiMessage extracts Cloudflare's structured error message, falling back
// to a fixed string rather than the raw body.
func apiMessage(data []byte) string {
	var env envelope
	if err := json.Unmarshal(data, &env); err == nil &&
		len(env.Errors) > 0 && env.Errors[0].Message != "" {
		return env.Errors[0].Message
	}
	return "non-2xx response (body withheld)"
}
