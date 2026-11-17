package fetch

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// capturingFlareStub records the decoded request payload it receives.
func capturingFlareStub(t *testing.T, got *flareRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok","message":"","solution":{"url":"https://target/","status":200,"response":"OK"}}`)
	}))
}

// When a proxy is configured, the FlareSolverr render must egress through it
// (FlareSolverr's own IP is blocked by sites like Reddit).
func TestFlareSolverrUsesProxyWhenConfigured(t *testing.T) {
	var got flareRequest
	flare := capturingFlareStub(t, &got)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.ProxyURL = "http://user:pass@proxy:80"
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if got.Proxy == nil {
		t.Fatal("expected proxy in FlareSolverr payload, got none")
	}
	// Chrome rejects credentials inside a proxy URL (ERR_NO_SUPPORTED_PROXIES),
	// so they must travel in FlareSolverr's separate username/password fields.
	if got.Proxy.URL != "http://proxy:80" {
		t.Errorf("proxy url = %q, want %q", got.Proxy.URL, "http://proxy:80")
	}
	if got.Proxy.Username != "user" || got.Proxy.Password != "pass" {
		t.Errorf("proxy credentials = %q/%q, want user/pass", got.Proxy.Username, got.Proxy.Password)
	}
}

// A proxy without credentials keeps its URL and sends no credential fields.
func TestFlareSolverrProxyWithoutCredentials(t *testing.T) {
	var got flareRequest
	flare := capturingFlareStub(t, &got)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.FlareSolverrProxyURL = "http://relay:8899"
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if got.Proxy == nil || got.Proxy.URL != "http://relay:8899" {
		t.Fatalf("proxy = %+v, want url http://relay:8899", got.Proxy)
	}
	if got.Proxy.Username != "" || got.Proxy.Password != "" {
		t.Errorf("expected no credentials, got %q/%q", got.Proxy.Username, got.Proxy.Password)
	}
}

// A schemeless proxy (user:pass@host:port) is treated as http and split the same way.
func TestFlareSolverrSchemelessProxyCredentials(t *testing.T) {
	var got flareRequest
	flare := capturingFlareStub(t, &got)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.ProxyURL = "user:p%40ss@proxy:80"
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if got.Proxy == nil || got.Proxy.URL != "http://proxy:80" {
		t.Fatalf("proxy = %+v, want url http://proxy:80", got.Proxy)
	}
	if got.Proxy.Username != "user" || got.Proxy.Password != "p@ss" {
		t.Errorf("proxy credentials = %q/%q, want user/p@ss", got.Proxy.Username, got.Proxy.Password)
	}
}

// The opt-out keeps the render direct even with a proxy configured.
func TestFlareSolverrNoProxyOptOut(t *testing.T) {
	var got flareRequest
	flare := capturingFlareStub(t, &got)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.ProxyURL = "http://user:pass@proxy:80"
	cfg.FlareSolverrNoProxy = true
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if got.Proxy != nil {
		t.Errorf("expected no proxy in payload (opt-out), got %q", got.Proxy.URL)
	}
}

// With no proxy configured the payload carries no proxy field.
func TestFlareSolverrNoProxyWhenUnset(t *testing.T) {
	var got flareRequest
	flare := capturingFlareStub(t, &got)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if got.Proxy != nil {
		t.Errorf("expected no proxy field, got %q", got.Proxy.URL)
	}
}
