package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// flareStub returns an httptest server that mimics a successful FlareSolverr
// request.get, serving the given rendered body. It records whether it was hit.
func flareStub(t *testing.T, body string, hit *bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hit != nil {
			*hit = true
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","message":"","solution":{"url":%q,"status":200,"response":%q}}`, "https://target/", body)
	}))
}

func testConfig() *Config {
	return &Config{Timeout: 5, MaxRetries: 0, UserAgent: DefaultUserAgent}
}

func TestFetchDirectSuccess(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "HELLO")
	}))
	defer target.Close()

	res := New(testConfig()).Fetch(target.URL, ViaAuto)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.Tier != TierDirect {
		t.Errorf("tier = %s, want direct", res.Tier)
	}
	if res.Status != 200 || string(res.Body) != "HELLO" {
		t.Errorf("got status=%d body=%q", res.Status, res.Body)
	}
}

func TestFetchChallengeEscalatesToFlareSolverr(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<html><head><title>Just a moment...</title></head></html>")
	}))
	defer target.Close()

	flareHit := false
	flare := flareStub(t, "SOLVED", &flareHit)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	res := New(cfg).Fetch(target.URL, ViaAuto)

	if !flareHit {
		t.Fatal("expected FlareSolverr to be invoked")
	}
	if res.Tier != TierFlareSolverr {
		t.Errorf("tier = %s, want flaresolverr", res.Tier)
	}
	if res.Status != 200 || string(res.Body) != "SOLVED" {
		t.Errorf("got status=%d body=%q", res.Status, res.Body)
	}
}

func TestFetchBlockedEscalatesToProxy(t *testing.T) {
	// Direct target rate-limits (IP block, no challenge markers).
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, "rate limited")
	}))
	defer target.Close()

	// Proxy stub intercepts the proxied request and serves a good response.
	proxyHit := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHit = true
		fmt.Fprint(w, "VIAPROXY")
	}))
	defer proxy.Close()

	cfg := testConfig()
	cfg.ProxyURL = proxy.URL
	res := New(cfg).Fetch(target.URL, ViaAuto)

	if !proxyHit {
		t.Fatal("expected proxy to be invoked")
	}
	if res.Tier != TierProxy {
		t.Errorf("tier = %s, want proxy", res.Tier)
	}
	if string(res.Body) != "VIAPROXY" {
		t.Errorf("got body=%q", res.Body)
	}
}

func TestFetchTerminalNoEscalation(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "nope")
	}))
	defer target.Close()

	flareHit := false
	flare := flareStub(t, "SOLVED", &flareHit)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.ProxyURL = "http://127.0.0.1:1" // would fail if used
	res := New(cfg).Fetch(target.URL, ViaAuto)

	if flareHit {
		t.Error("FlareSolverr should not be invoked for a 404")
	}
	if res.Tier != TierDirect || res.Status != 404 {
		t.Errorf("got tier=%s status=%d, want direct/404", res.Tier, res.Status)
	}
}

func TestFetchNoFallback(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<title>Just a moment...</title>")
	}))
	defer target.Close()

	flareHit := false
	flare := flareStub(t, "SOLVED", &flareHit)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	res := New(cfg).Fetch(target.URL, ViaDirect)

	if flareHit {
		t.Error("FlareSolverr should not be invoked with via=direct")
	}
	if res.Tier != TierDirect || res.Status != 403 {
		t.Errorf("got tier=%s status=%d, want direct/403", res.Tier, res.Status)
	}
}

func TestForceViaProxy(t *testing.T) {
	// Direct would succeed, but --via proxy must route through the proxy anyway.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "DIRECT")
	}))
	defer target.Close()

	proxyHit := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHit = true
		fmt.Fprint(w, "VIAPROXY")
	}))
	defer proxy.Close()

	cfg := testConfig()
	cfg.ProxyURL = proxy.URL
	res := New(cfg).Fetch(target.URL, ViaProxy)

	if !proxyHit || res.Tier != TierProxy || string(res.Body) != "VIAPROXY" {
		t.Errorf("forced proxy not used: hit=%v tier=%s body=%q", proxyHit, res.Tier, res.Body)
	}
}

func TestForceViaProxyUnconfigured(t *testing.T) {
	res := New(testConfig()).Fetch("https://example.com", ViaProxy)
	if res.Err == nil {
		t.Error("expected error when forcing proxy with no proxy configured")
	}
}

func TestForceViaFlareSolverrUnconfigured(t *testing.T) {
	res := New(testConfig()).Fetch("https://example.com", ViaFlareSolverr)
	if res.Err == nil {
		t.Error("expected error when forcing flaresolverr with none configured")
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		r    *Result
		want failureKind
	}{
		{"ok", &Result{Status: 200}, kindOK},
		{"transport error", &Result{Err: fmt.Errorf("boom")}, kindBlocked},
		{"plain 403", &Result{Status: 403}, kindBlocked},
		{"429", &Result{Status: 429}, kindBlocked},
		{"500", &Result{Status: 500}, kindBlocked},
		{"404 terminal", &Result{Status: 404}, kindTerminal},
		{"401 terminal", &Result{Status: 401}, kindTerminal},
		{"cf challenge body", &Result{Status: 403, Body: []byte("<title>Just a moment</title>")}, kindChallenge},
		{"cf-mitigated header", &Result{Status: 503, cfMitigated: true}, kindChallenge},
		// A 200 bot-verification interstitial (e.g. Reddit) must escalate.
		{"200 reddit interstitial", &Result{Status: 200, Body: []byte("<title>Reddit - Please wait for verification</title>")}, kindChallenge},
		{"200 cf-mitigated", &Result{Status: 200, cfMitigated: true}, kindChallenge},
		// A legitimate 200 page that merely contains a loose marker phrase must
		// NOT be misrouted to the renderer (strong markers only on 2xx).
		{"200 loose marker stays ok", &Result{Status: 200, Body: []byte("<p>Attention required for this form</p>")}, kindOK},
	}
	for _, c := range cases {
		if got := classify(c.r); got != c.want {
			t.Errorf("%s: classify = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestNormalizeProxyURL(t *testing.T) {
	if got := normalizeProxyURL("user:pass@host:80"); got != "http://user:pass@host:80" {
		t.Errorf("schemeless: got %q", got)
	}
	if got := normalizeProxyURL("http://host:80"); got != "http://host:80" {
		t.Errorf("with scheme: got %q", got)
	}
}
