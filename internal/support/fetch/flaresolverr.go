package fetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// flareRequest is a FlareSolverr v1 request.get payload.
type flareRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url"`
	MaxTimeout int    `json:"maxTimeout"` // milliseconds
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

// attemptFlareSolverr proxies the request through FlareSolverr, which renders
// the page in a headless browser and solves Cloudflare/JS challenges.
func (f *Fetcher) attemptFlareSolverr(rawURL string) *Result {
	res := &Result{URL: rawURL, Tier: TierFlareSolverr}

	endpoint := strings.TrimRight(f.cfg.FlareSolverrURL, "/") + "/v1"
	maxTimeout := f.cfg.Timeout * 1000
	if maxTimeout <= 0 {
		maxTimeout = DefaultTimeout * 1000
	}

	payload, err := json.Marshal(flareRequest{Cmd: "request.get", URL: rawURL, MaxTimeout: maxTimeout})
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
