// Package openrouter is the OpenRouter provider: the operator's management
// key mints one spend-capped API key per session and revokes it at destroy.
// Plain net/http — OpenRouter ships no official Go SDK and two endpoints
// don't warrant a third-party one.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// BaseURL is the key-management API base URL (POST/DELETE /keys). Exported
// so cli e2e tests can point mint/revoke at an httptest server.
var BaseURL = "https://openrouter.ai/api/v1"

// httpClient issues every request; mint/revoke are bounded by ctx.
var httpClient = http.DefaultClient

// mintRequest is the POST /keys body: name becomes the dashboard label,
// limit the USD spend cap. Every other documented field is unused here.
type mintRequest struct {
	Name  string  `json:"name"`
	Limit float64 `json:"limit"`
}

// mintResponse decodes only the durable id. The plaintext key in the same
// response is deliberately never decoded: nothing consumes it yet.
type mintResponse struct {
	Data struct {
		Hash string `json:"hash"`
	} `json:"data"`
}

// apiError is OpenRouter's error envelope on a non-2xx response.
type apiError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// statusError carries the HTTP status so revoke can treat 404 as success.
type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string { return fmt.Sprintf("status %d: %s", e.code, e.msg) }

// validate checks the management key with a read-only key list; any 2xx
// proves it authenticates.
func validate(ctx context.Context, managementKey string) error {
	if err := do(ctx, http.MethodGet, "/keys", managementKey, nil, nil); err != nil {
		return fmt.Errorf("openrouter: validate key: %w", err)
	}
	return nil
}

// mint creates one spend-capped API key named label; the response's key
// value is discarded — only the revoke handle is returned.
func mint(ctx context.Context, managementKey string, capUSD float64,
	label string) (string, error) {
	body, err := json.Marshal(mintRequest{Name: label, Limit: capUSD})
	if err != nil {
		return "", fmt.Errorf("openrouter: mint: encode request: %w", err)
	}

	var out mintResponse
	if err := do(ctx, http.MethodPost, "/keys", managementKey, body, &out); err != nil {
		return "", fmt.Errorf("openrouter: mint: %w", err)
	}
	if out.Data.Hash == "" {
		return "", fmt.Errorf("openrouter: mint: response missing key id")
	}
	return out.Data.Hash, nil
}

// revoke deletes the key identified by hash. A 404 is success: the key is
// already gone, so a re-run destroy stays clean.
func revoke(ctx context.Context, managementKey, hash string) error {
	err := do(ctx, http.MethodDelete, "/keys/"+url.PathEscape(hash), managementKey, nil, nil)
	var se *statusError
	if errors.As(err, &se) && se.code == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("openrouter: revoke: %w", err)
	}
	return nil
}

// do runs one request under BaseURL, Bearer-authenticated. Errors never
// include the management key or a raw response body — the structured
// message is extracted instead (a malformed-request echo would leak the
// key just sent in it).
func do(ctx context.Context, method, path, managementKey string, body []byte, out any) error {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, BaseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+managementKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &statusError{code: resp.StatusCode, msg: apiErrorMessage(data)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// apiErrorMessage extracts OpenRouter's structured message, falling back to
// a fixed string rather than the raw body.
func apiErrorMessage(data []byte) string {
	var apiErr apiError
	if err := json.Unmarshal(data, &apiErr); err == nil && apiErr.Error.Message != "" {
		return apiErr.Error.Message
	}
	return "non-2xx response (body withheld)"
}
