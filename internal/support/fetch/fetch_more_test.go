package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Forcing --via flaresolverr renders through the headless browser regardless of
// the direct response (the path that handles JavaScript-rendered pages).
func TestForceViaFlareSolverrSuccess(t *testing.T) {
	flareHit := false
	flare := flareStub(t, "RENDERED", &flareHit)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	res := New(cfg).Fetch("https://target/", ViaFlareSolverr)

	if !flareHit {
		t.Fatal("expected FlareSolverr to be invoked")
	}
	if res.Tier != TierFlareSolverr || string(res.Body) != "RENDERED" {
		t.Errorf("tier=%s body=%q", res.Tier, res.Body)
	}
}

// A direct transport error (kindBlocked) with no proxy falls back to
// FlareSolverr as a last resort.
func TestFetchTransportErrorFallsBackToFlareSolverr(t *testing.T) {
	// A closed server URL produces a connection error on the direct attempt.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	flareHit := false
	flare := flareStub(t, "LASTRESORT", &flareHit)
	defer flare.Close()

	cfg := testConfig()
	cfg.FlareSolverrURL = flare.URL
	res := New(cfg).Fetch(deadURL, ViaAuto)

	if !flareHit {
		t.Fatal("expected FlareSolverr last-resort invocation")
	}
	if string(res.Body) != "LASTRESORT" {
		t.Errorf("got body=%q", res.Body)
	}
}

// doHTTP retries on 5xx up to MaxRetries, then returns the last response.
func TestDoHTTPRetriesOn5xx(t *testing.T) {
	var calls int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, "upstream down")
	}))
	defer target.Close()

	cfg := testConfig()
	cfg.MaxRetries = 1 // one retry → two total attempts
	res := New(cfg).Fetch(target.URL, ViaDirect)

	if calls != 2 {
		t.Errorf("server hit %d times, want 2 (initial + 1 retry)", calls)
	}
	if res.Status != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", res.Status)
	}
}
