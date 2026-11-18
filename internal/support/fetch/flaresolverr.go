package fetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// flareProxy routes the FlareSolverr browser's egress through an HTTP proxy.
// Credentials go in Username/Password, never inside URL: Chrome rejects a
// proxy URL that carries user:pass (ERR_NO_SUPPORTED_PROXIES), while
// FlareSolverr answers the proxy's auth challenge itself from these fields.
type flareProxy struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// newFlareProxy splits any user:pass out of rawURL into the separate fields.
// An unparseable URL is passed through unchanged.
func newFlareProxy(rawURL string) *flareProxy {
	parsed, err := url.Parse(normalizeProxyURL(rawURL))
	if err != nil || parsed.User == nil {
		return &flareProxy{URL: rawURL}
	}
	p := &flareProxy{Username: parsed.User.Username()}
	p.Password, _ = parsed.User.Password()
	parsed.User = nil
	p.URL = parsed.String()
	return p
}

// flareRequest is a FlareSolverr v1 request.get payload.
type flareRequest struct {
	Cmd        string      `json:"cmd"`
	URL        string      `json:"url"`
	MaxTimeout int         `json:"maxTimeout"` // milliseconds
	Proxy      *flareProxy `json:"proxy,omitempty"`
}

// flareResponse is the relevant subset of a FlareSolverr v1 response.
type flareResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Solution struct {
		URL      string `json:"url"`
		Status   int    `json:"status"`
		Response string `json:"response"`
	} `json:"solution"`
}

// flareOverhead is the extra time allowed beyond the target maxTimeout for
// FlareSolverr to spin up the browser and solve the challenge.
const flareOverhead = 30 * time.Second

// renderFailureMarkers flag a FlareSolverr-rendered body that is itself an error
// or block page — a Chrome proxy error, an unreachable page, or a residual block.
// Such a render is worth one more attempt on a freshly rotated proxy IP.
var renderFailureMarkers = []string{
	"err_no_supported_proxies",
	"err_proxy_connection_failed",
	"err_tunnel_connection_failed",
	"this site can’t be reached", // curly apostrophe (Chrome error page)
	"this site can't be reached",
	"blocked by network security",
}

// flareProxyURL is the proxy the FlareSolverr render should egress through: a
// dedicated FlareSolverr proxy when set, otherwise the shared proxy.
func (f *Fetcher) flareProxyURL() string {
	if f.cfg.FlareSolverrNoProxy {
		return ""
	}
	if f.cfg.FlareSolverrProxyURL != "" {
		return f.cfg.FlareSolverrProxyURL
	}
	return f.cfg.ProxyURL
}

// attemptFlareSolverrRetry renders through FlareSolverr, retrying within
// MaxRetries when the rendered body is itself an error/block page (a fresh
// rotated proxy IP or browser often succeeds on the next try).
func (f *Fetcher) attemptFlareSolverrRetry(rawURL string) *Result {
	var res *Result
	for attempt := 0; attempt <= f.cfg.MaxRetries; attempt++ {
		res = f.attemptFlareSolverr(rawURL)
		if res.Err == nil && !renderFailed(res.Body) {
			return res
		}
		if attempt < f.cfg.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}
	return res
}

// renderFailed reports whether a rendered body is an error/block interstitial.
func renderFailed(body []byte) bool {
	return isChallengeBody(body, renderFailureMarkers)
}

// attemptFlareSolverr proxies the request through FlareSolverr, which renders
// the page in a headless browser and solves Cloudflare/JS challenges.
func (f *Fetcher) attemptFlareSolverr(rawURL string) *Result {
	res := &Result{URL: rawURL, Tier: TierFlareSolverr}

	endpoint := strings.TrimRight(f.cfg.FlareSolverrURL, "/") + "/v1"
	maxTimeout := f.cfg.Timeout * 1000
	if maxTimeout <= 0 {
		maxTimeout = DefaultTimeout * 1000
	}

	reqBody := flareRequest{Cmd: "request.get", URL: rawURL, MaxTimeout: maxTimeout}
	// Route the headless browser through the proxy too: some sites (e.g. Reddit)
	// block FlareSolverr's own datacenter IP, so a direct render still fails.
	if proxy := f.flareProxyURL(); proxy != "" {
		reqBody.Proxy = newFlareProxy(proxy)
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		res.Err = fmt.Errorf("flaresolverr marshal failed: %w", err)
		return res
	}

	client := &http.Client{Timeout: f.timeout() + flareOverhead}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		res.Err = fmt.Errorf("flaresolverr request build failed: %w", err)
		return res
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		res.Err = fmt.Errorf("flaresolverr unreachable: %w", err)
		return res
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Err = fmt.Errorf("flaresolverr read failed: %w", err)
		return res
	}

	var fr flareResponse
	if err := json.Unmarshal(raw, &fr); err != nil {
		res.Err = fmt.Errorf("flaresolverr bad response: %w", err)
		res.Status = resp.StatusCode
		return res
	}
	if fr.Status != "ok" {
		res.Err = fmt.Errorf("flaresolverr error: %s", fr.Message)
		res.Status = fr.Solution.Status
		return res
	}

	res.Status = fr.Solution.Status
	res.FinalURL = fr.Solution.URL
	res.Body = []byte(fr.Solution.Response)
	res.Bytes = len(res.Body)
	res.ContentType = "text/html" // FlareSolverr returns rendered HTML
	return res
}
