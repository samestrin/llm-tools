package fetch

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Tier identifies which path produced a result.
type Tier string

const (
	TierDirect       Tier = "direct"
	TierProxy        Tier = "proxy"
	TierFlareSolverr Tier = "flaresolverr"
)

// Via selects how Fetch routes a request.
const (
	ViaAuto         = "auto"         // full fallback ladder (default)
	ViaDirect       = "direct"       // direct tier only
	ViaProxy        = "proxy"        // force the rotating proxy
	ViaFlareSolverr = "flaresolverr" // force the renderer
)

// Result is the outcome of a fetch attempt.
type Result struct {
	URL         string
	FinalURL    string
	Status      int
	Tier        Tier
	Bytes       int
	Body        []byte
	ContentType string
	Err         error

	// cfMitigated records the cf-mitigated response header, used by classify.
	cfMitigated bool
}

// failureKind drives the fallback ladder.
type failureKind int

const (
	kindOK        failureKind = iota // good response — stop
	kindBlocked                      // IP-level block — escalate to proxy
	kindChallenge                    // Cloudflare/JS challenge — escalate to FlareSolverr
	kindTerminal                     // do not escalate (404, 401, 410, ...)
)

// challengeMarkers are case-insensitive substrings that mark a Cloudflare/JS
// interstitial challenge page. Applied when the status code already signals a
// block (403/429/503), so looser natural-language entries are acceptable.
var challengeMarkers = []string{
	"just a moment",
	"cf-browser-verification",
	"challenge-platform",
	"_cf_chl_opt",
	"cf_chl_",
	"attention required",
}

// strongChallengeMarkers are high-confidence interstitial fingerprints, safe to
// act on even for a 200 response — some sites (e.g. Reddit) serve a bot
// verification wall with a 200 status. Kept narrower than challengeMarkers so a
// legitimate 2xx page that merely contains a phrase like "attention required"
// is not misrouted to the renderer.
var strongChallengeMarkers = []string{
	"cf-browser-verification",
	"challenge-platform",
	"_cf_chl_opt",
	"cf_chl_",
	"please wait for verification", // Reddit interstitial
}

// Fetcher runs the fallback ladder for a Config.
type Fetcher struct {
	cfg *Config
}

// New builds a Fetcher. A nil cfg loads configuration from the environment.
func New(cfg *Config) *Fetcher {
	if cfg == nil {
		cfg = LoadConfig()
	}
	return &Fetcher{cfg: cfg}
}

// Fetch retrieves rawURL using the given routing mode (see the Via constants).
//
// ViaAuto runs the smart, failure-type-routed ladder:
//
//	direct ──403/429/5xx/conn──▶ proxy ──JS challenge──▶ flaresolverr
//	       ──JS challenge───────────────────────────────▶ flaresolverr
//
// ViaDirect/ViaProxy/ViaFlareSolverr force exactly that single tier; a forced
// tier whose endpoint is unconfigured returns an error result.
func (f *Fetcher) Fetch(rawURL string, via string) *Result {
	switch via {
	case ViaDirect:
		return f.attemptDirect(rawURL)
	case ViaProxy:
		if f.cfg.ProxyURL == "" {
			return &Result{URL: rawURL, Tier: TierProxy, Err: fmt.Errorf("--via proxy requires a proxy URL (FETCH_PROXY_URL or --proxy)")}
		}
		return f.attemptProxy(rawURL)
	case ViaFlareSolverr:
		if f.cfg.FlareSolverrURL == "" {
			return &Result{URL: rawURL, Tier: TierFlareSolverr, Err: fmt.Errorf("--via flaresolverr requires a FlareSolverr URL (FETCH_FLARESOLVERR_URL or --flaresolverr)")}
		}
		return f.attemptFlareSolverrRetry(rawURL)
	}

	// ViaAuto (and any unspecified value): full ladder.
	res := f.attemptDirect(rawURL)

	switch classify(res) {
	case kindOK, kindTerminal:
		return res

	case kindChallenge:
		if f.cfg.FlareSolverrURL != "" {
			return f.attemptFlareSolverrRetry(rawURL)
		}
		return res

	case kindBlocked:
		if f.cfg.ProxyURL != "" {
			pres := f.attemptProxy(rawURL)
			switch classify(pres) {
			case kindOK, kindTerminal:
				return pres
			case kindChallenge:
				if f.cfg.FlareSolverrURL != "" {
					return f.attemptFlareSolverrRetry(rawURL)
				}
				return pres
			}
			res = pres // still blocked after proxy
		}
		// Last resort: a renderer can sometimes pass where raw IPs are blocked.
		if f.cfg.FlareSolverrURL != "" {
			return f.attemptFlareSolverrRetry(rawURL)
		}
		return res
	}

	return res
}

func (f *Fetcher) timeout() time.Duration {
	t := f.cfg.Timeout
	if t <= 0 {
		t = DefaultTimeout
	}
	return time.Duration(t) * time.Second
}

func (f *Fetcher) attemptDirect(rawURL string) *Result {
	client := &http.Client{Timeout: f.timeout()}
	return f.doHTTP(client, rawURL, TierDirect)
}

func (f *Fetcher) attemptProxy(rawURL string) *Result {
	client, err := proxyClient(f.cfg.ProxyURL, f.timeout())
	if err != nil {
		// Never wrap the parse error: url.Parse echoes the raw URL, which carries
		// the proxy credentials (user:pass). Surface a credential-free message.
		return &Result{URL: rawURL, Tier: TierProxy, Err: fmt.Errorf("invalid proxy URL (check FETCH_PROXY_URL / --proxy)")}
	}
	return f.doHTTP(client, rawURL, TierProxy)
}

// doHTTP runs a single tier with retries on transport errors and 5xx responses.
func (f *Fetcher) doHTTP(client *http.Client, rawURL string, tier Tier) *Result {
	var res *Result
	for attempt := 0; attempt <= f.cfg.MaxRetries; attempt++ {
		res = f.doHTTPOnce(client, rawURL, tier)
		// 4xx and 2xx are final for this tier; only retry transport errors / 5xx.
		if res.Err == nil && res.Status < 500 {
			return res
		}
		if attempt < f.cfg.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}
	return res
}

func (f *Fetcher) doHTTPOnce(client *http.Client, rawURL string, tier Tier) *Result {
	res := &Result{URL: rawURL, Tier: tier}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		res.Err = fmt.Errorf("failed to create request: %w", err)
		return res
	}
	req.Header.Set("User-Agent", f.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		res.Err = err
		return res
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	res.Status = resp.StatusCode
	res.FinalURL = resp.Request.URL.String()
	res.ContentType = resp.Header.Get("Content-Type")
	res.cfMitigated = resp.Header.Get("cf-mitigated") != ""
	if err != nil {
		res.Err = fmt.Errorf("failed to read body: %w", err)
		return res
	}
	res.Body = body
	res.Bytes = len(body)
	return res
}

// classify maps a result to a failure kind for routing.
func classify(r *Result) failureKind {
	if r.Err != nil {
		return kindBlocked // transport failure — a different egress may work
	}
	if r.Status >= 200 && r.Status < 300 {
		// Some sites (e.g. Reddit) serve a bot-verification interstitial with a
		// 200 status. Escalate to the renderer only on a high-confidence
		// fingerprint, so ordinary 2xx pages are never misrouted.
		if r.cfMitigated || isChallengeBody(r.Body, strongChallengeMarkers) {
			return kindChallenge
		}
		return kindOK
	}

	// Challenge pages arrive as 403/503/429 carrying Cloudflare fingerprints.
	if r.cfMitigated || isChallengeBody(r.Body, challengeMarkers) {
		switch r.Status {
		case http.StatusForbidden, http.StatusServiceUnavailable, http.StatusTooManyRequests:
			return kindChallenge
		}
	}

	switch {
	case r.Status == http.StatusForbidden, r.Status == http.StatusTooManyRequests:
		return kindBlocked
	case r.Status >= 500:
		return kindBlocked
	default:
		return kindTerminal // 401, 404, 410, ...
	}
}

func isChallengeBody(body []byte, markers []string) bool {
	if len(body) == 0 {
		return false
	}
	n := len(body)
	if n > 4096 {
		n = 4096
	}
	lower := strings.ToLower(string(body[:n]))
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
