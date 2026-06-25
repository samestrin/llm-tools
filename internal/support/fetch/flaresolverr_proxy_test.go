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
	if got.Proxy.URL != cfg.ProxyURL {
		t.Errorf("proxy url = %q, want %q", got.Proxy.URL, cfg.ProxyURL)
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
