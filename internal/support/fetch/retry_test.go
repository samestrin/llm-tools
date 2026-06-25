package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// flareSeqStub serves a sequence of rendered bodies (one per request), so a
// test can simulate a failed render followed by a good one.
func flareSeqStub(t *testing.T, bodies []string, calls *int) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		i := *calls
		*calls++
		mu.Unlock()
		body := bodies[len(bodies)-1]
		if i < len(bodies) {
			body = bodies[i]
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","message":"","solution":{"url":%q,"status":200,"response":%q}}`, "https://target/", body)
	}))
}

// A failed render (Chrome proxy error) is retried; the next good render wins.
func TestFlareSolverrRetriesFailedRender(t *testing.T) {
	calls := 0
	flare := flareSeqStub(t, []string{
		"<html><body>ERR_NO_SUPPORTED_PROXIES</body></html>",
		"<html><body>REAL CONTENT</body></html>",
	}, &calls)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.MaxRetries = 2
	res := New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if calls != 2 {
		t.Errorf("flaresolverr called %d times, want 2 (1 failed + 1 good)", calls)
	}
	if string(res.Body) != "<html><body>REAL CONTENT</body></html>" {
		t.Errorf("got body %q, want the good render", res.Body)
	}
}

// When every render fails, the last result is returned (retries exhausted).
func TestFlareSolverrRetryExhausted(t *testing.T) {
	calls := 0
	flare := flareSeqStub(t, []string{"<html>blocked by network security</html>"}, &calls)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.MaxRetries = 1
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if calls != 2 {
		t.Errorf("flaresolverr called %d times, want 2 (initial + 1 retry)", calls)
	}
}

// A good render on the first try is not retried.
func TestFlareSolverrNoRetryOnSuccess(t *testing.T) {
	calls := 0
	flare := flareSeqStub(t, []string{"<html>fine</html>"}, &calls)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.MaxRetries = 3
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if calls != 1 {
		t.Errorf("flaresolverr called %d times, want 1", calls)
	}
}

// The dedicated FlareSolverr proxy takes precedence over the shared proxy for
// the render, while leaving the shared proxy for the direct tier.
func TestFlareSolverrProxyPreference(t *testing.T) {
	var got flareRequest
	flare := capturingFlareStub(t, &got)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.ProxyURL = "http://user:pass@shared:80"
	cfg.FlareSolverrProxyURL = "http://relay:8080"
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if got.Proxy == nil || got.Proxy.URL != "http://relay:8080" {
		t.Errorf("render proxy = %v, want the dedicated relay", got.Proxy)
	}
}

// FlareSolverrNoProxy overrides even a dedicated FlareSolverr proxy.
func TestFlareSolverrNoProxyOverridesDedicated(t *testing.T) {
	var got flareRequest
	flare := capturingFlareStub(t, &got)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	cfg.FlareSolverrProxyURL = "http://relay:8080"
	cfg.FlareSolverrNoProxy = true
	New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if got.Proxy != nil {
		t.Errorf("expected no render proxy (opt-out), got %q", got.Proxy.URL)
	}
}
